package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"github.com/PuerkitoBio/fetchbot"
	"github.com/PuerkitoBio/goquery"
	"github.com/yhat/scrape"
)

// Sources struct to model a Type we want to ingest into the elasticsearch index
// and the links we want to crawl/scrape information to store in our index/type
type Sources struct {
	Type  string
	Links []string
}

// Document stuct to model our single "Document" store we will ingestion into the
// elasticsearch index/type
type Document struct {
	title   string
	content string
	link    string
}

// IndexType struct to model our ingestion set for multiple types and Documents
// for our index
type IndexType struct {
	DocType   string
	Documents []Document
}

// Index struct to model each index ingestion set for our elasticsearch data
type Index struct {
	Index string
	Type  IndexType
}

var (
	// Protect access to dup
	mu sync.Mutex
	// Duplicates table
	dup          = map[string]bool{}
	ingestionSet IndexType

	// Command-line flags
	cancelAfter = flag.Duration("cancelafter", 0, "automatically cancel the fetchbot after a given time")
	cancelAtURL = flag.String("cancelat", "", "automatically cancel the fetchbot at a given URL")
	stopAfter   = flag.Duration("stopafter", 0, "automatically stop the fetchbot after a given time")
	stopAtURL   = flag.String("stopat", "", "automatically stop the fetchbot at a given URL")
	memStats    = flag.Duration("memstats", 0, "display memory statistics at a given interval")
)

func main() {
	// parse the link json file to pass into the crawler
	src := parseJSON()

	// start the crawler
	for _, s := range src.Links {
		u, err := url.Parse(s)
		if err != nil {
			log.Fatal(err)
		}

		crawl(u.String(), *u)
	}

}

func parseJSON() Sources {
	var s Sources
	data, errRead := ioutil.ReadFile("./data.json")
	if errRead != nil {
		panic(errRead)
	}

	errUnmarshal := json.Unmarshal(data, &s)
	if errUnmarshal != nil {
		panic(errUnmarshal)
	}

	return s
}

func crawl(url string, u url.URL) {
	flag.Parse()

	// Create the muxer
	mux := fetchbot.NewMux()

	// Handle all errors the same
	mux.HandleErrors(fetchbot.HandlerFunc(func(ctx *fetchbot.Context, res *http.Response, err error) {
		fmt.Printf("[ERR] %s %s - %s\n", ctx.Cmd.Method(), ctx.Cmd.URL(), err)
	}))

	// Handle GET requests for html responses, to parse the body and enqueue all links as HEAD
	// requests.
	mux.Response().Method("GET").ContentType("text/html").Handler(fetchbot.HandlerFunc(
		func(ctx *fetchbot.Context, res *http.Response, err error) {
			// Process the body to find the links
			doc, err := goquery.NewDocumentFromResponse(res)
			if err != nil {
				fmt.Printf("[ERR] %s %s - %s\n", ctx.Cmd.Method(), ctx.Cmd.URL(), err)
				return
			}
			// Enqueue all links as HEAD requests
			enqueueLinks(ctx, doc)
		}))

	// Handle HEAD requests for html responses coming from the source host - we don't want
	// to crawl links from other hosts.
	mux.Response().Method("HEAD").Host(u.Host).ContentType("text/html").Handler(fetchbot.HandlerFunc(
		func(ctx *fetchbot.Context, res *http.Response, err error) {
			if _, err := ctx.Q.SendStringGet(ctx.Cmd.URL().String()); err != nil {
				fmt.Printf("[ERR] %s %s - %s\n", ctx.Cmd.Method(), ctx.Cmd.URL(), err)
			}
		}))

	// Create the Fetcher, handle the logging first, then dispatch to the Muxer
	h := scrapeHandler(mux)
	if *stopAtURL != "" || *cancelAtURL != "" {
		stopURL := *stopAtURL
		if *cancelAtURL != "" {
			stopURL = *cancelAtURL
		}
		h = stopHandler(stopURL, *cancelAtURL != "", scrapeHandler(mux))
	}
	f := fetchbot.New(h)

	// Start processing
	q := f.Start()

	// if a stop or cancel is requested after some duration, launch the goroutine
	// that will stop or cancel.
	if *stopAfter > 0 || *cancelAfter > 0 {
		after := *stopAfter
		stopFunc := q.Close
		if *cancelAfter != 0 {
			after = *cancelAfter
			stopFunc = q.Cancel
		}

		go func() {
			c := time.After(after)
			<-c
			stopFunc()
		}()
	}

	// Enqueue the seed, which is the first entry in the dup map
	dup[url] = true
	_, err := q.SendStringGet(url)
	if err != nil {
		fmt.Printf("[ERR] GET %s - %s\n", url, err)
	}
	q.Block()
}

// stopHandler stops the fetcher if the stopurl is reached. Otherwise it dispatches
// the call to the wrapped Handler.
func stopHandler(stopurl string, cancel bool, wrapped fetchbot.Handler) fetchbot.Handler {
	return fetchbot.HandlerFunc(func(ctx *fetchbot.Context, res *http.Response, err error) {
		if ctx.Cmd.URL().String() == stopurl {
			fmt.Printf(">>>>> STOP URL %s\n", ctx.Cmd.URL())
			// generally not a good idea to stop/block from a handler goroutine
			// so do it in a separate goroutine
			go func() {
				if cancel {
					ctx.Q.Cancel()
				} else {
					ctx.Q.Close()
				}
			}()
			return
		}
		wrapped.Handle(ctx, res, err)
	})
}

// scrapeHandler will fire a scraper method on the page if successful response,
// append the scraped document stored for index ingestion
// and dispatches the call to the wrapped Handler.
func scrapeHandler(wrapped fetchbot.Handler) fetchbot.Handler {
	return fetchbot.HandlerFunc(func(ctx *fetchbot.Context, res *http.Response, err error) {
		if err == nil {
			if res.StatusCode == 200 {
				url := ctx.Cmd.URL().String()
				response := scraper(url)
				// TODO: store/log the bad sites with null fields
				if response.content != "" && response.title != "" && response.link != "" {
					fmt.Println("New page was scraped!")
					fmt.Println(response)
					ingestionSet.Documents = append(ingestionSet.Documents, response)
					fmt.Println("Total Pages Scraped Successfully: ", len(ingestionSet.Documents))
				}
			}
			// fmt.Printf("[%d] %s %s - %s\n", res.StatusCode, ctx.Cmd.Method(), ctx.Cmd.URL(), res.Header.Get("Content-Type"))
		}
		wrapped.Handle(ctx, res, err)
	})
}

func enqueueLinks(ctx *fetchbot.Context, doc *goquery.Document) {
	mu.Lock()
	doc.Find("a[href]").Each(func(i int, s *goquery.Selection) {
		val, _ := s.Attr("href")
		// Resolve address
		u, err := ctx.Cmd.URL().Parse(val)
		if err != nil {
			fmt.Printf("error: resolve URL %s - %s\n", val, err)
			return
		}
		if !dup[u.String()] {
			if _, err := ctx.Q.SendStringHead(u.String()); err != nil {
				fmt.Printf("error: enqueue head %s - %s\n", u, err)
			} else {
				dup[u.String()] = true
			}
		}
	})
	mu.Unlock()
}

func respGenerator(url string) <-chan *http.Response {
	var wg sync.WaitGroup
	out := make(chan *http.Response)
	wg.Add(1)

	go func(url string) {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			panic(err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			panic(err)
		}
		out <- resp
		wg.Done()
	}(url)

	go func() {
		wg.Wait()
		close(out)
	}()
	return out
}

func rootGenerator(in <-chan *http.Response) <-chan *html.Node {
	var wg sync.WaitGroup
	out := make(chan *html.Node)
	for resp := range in {
		wg.Add(1)
		go func(resp *http.Response) {
			root, err := html.Parse(resp.Body)
			if err != nil {
				panic(err)
			}
			out <- root
			wg.Done()
		}(resp)
	}
	go func() {
		wg.Wait()
		close(out)
	}()
	return out
}

// paragraphGenerator functino will take in a channel with a pointer to an html.Node
// type it will use scrape's ByTag API function and scrape all the Title tags from
// the Node and return a channel with a type string
func titleGenerator(in <-chan *html.Node) <-chan string {
	var wg sync.WaitGroup
	out := make(chan string)
	for root := range in {
		wg.Add(1)
		go func(root *html.Node) {
			title, ok := scrape.Find(root, scrape.ByTag(atom.Title))
			if ok {
				out <- scrape.Text(title)
			}
			wg.Done()
		}(root)
	}
	go func() {
		wg.Wait()
		close(out)
	}()
	return out
}

// paragraphGenerator functino will take in a channel with a pointer to an html.Node
// type it will use scrape's ByTag API function and scrape all the P tags from
// the Node and return a channel with a type string
func paragraphGenerator(in <-chan *html.Node) <-chan string {
	var wg sync.WaitGroup
	out := make(chan string)
	for root := range in {
		wg.Add(1)
		go func(root *html.Node) {
			elements := scrape.FindAll(root, scrape.ByTag(atom.P))
			for _, element := range elements {
				if element != nil {
					out <- scrape.Text(element)
				}
			}
			wg.Done()
		}(root)
	}
	go func() {
		wg.Wait()
		close(out)
	}()
	return out
}

// rowGenerator function will take in a channel with a pointer to an html.Node
// type it will use scrape's ByTag API function and scrape all the Content tags from
// the Node and return a channel with a type string
func rowsGenerator(in <-chan *html.Node) <-chan string {
	var wg sync.WaitGroup
	out := make(chan string)
	for root := range in {
		wg.Add(1)
		go func(root *html.Node) {
			elements := scrape.FindAll(root, scrape.ByTag(atom.Content))
			for _, element := range elements {
				if element != nil {
					out <- scrape.Text(element)
				}
			}
			wg.Done()
		}(root)
	}
	go func() {
		wg.Wait()
		close(out)
	}()
	return out
}

// scraper function will take a url and fire off pipelines to scrape titles,
// paragraphs, divs and return a Document struct with valid title, content and a link
func scraper(url string) Document {
	var doc Document

	contents := make([]string, 0)

	for title := range titleGenerator(rootGenerator(respGenerator(url))) {
		doc.title = title
	}
	for para := range paragraphGenerator(rootGenerator(respGenerator(url))) {
		contents = append(contents, para)
	}

	for row := range rowsGenerator(rootGenerator(respGenerator(url))) {
		contents = append(contents, row)
	}

	// combine all the paragraphs from the page
	content := strings.Join(contents, " ")
	doc.content = content
	doc.link = url

	return doc
}
