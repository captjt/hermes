package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"runtime"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"github.com/PuerkitoBio/fetchbot"
	"github.com/PuerkitoBio/goquery"
	"github.com/yhat/scrape"
)

type (
	// Sources struct to model a Type we want to ingest into the elasticsearch index
	// and the links we want to crawl/scrape information to store in our index/type
	Sources struct {
		Type  string
		Links []string
	}

	// Document stuct to model our single "Document" store we will ingestion into the
	// elasticsearch index/type
	Document struct {
		title   string
		content string
		link    string
	}

	// IndexType struct to model our ingestion set for multiple types and Documents
	// for our index
	IndexType struct {
		DocType   string
		Documents []Document
	}

	// Index struct to model each index ingestion set for our elasticsearch data
	Index struct {
		Index string
		Type  IndexType
	}
)

var (
	// Protect access to dup
	mu sync.Mutex
	// Duplicates table
	dup = map[string]bool{}

	// ingestion data TODO make non global
	ingestionSet IndexType

	// bad links TODO make non global
	badLinks []string

	// Command-line flags
	cancelAfter   = flag.Duration("cancelafter", 0, "automatically cancel the fetchbot after a given time")
	cancelAtURL   = flag.String("cancelat", "", "automatically cancel the fetchbot at a given URL")
	stopAfter     = flag.Duration("stopafter", time.Minute/2, "automatically stop the fetchbot after a given time")
	stopAtURL     = flag.String("stopat", "", "automatically stop the fetchbot at a given URL")
	memStats      = flag.Duration("memstats", time.Minute/4, "display memory statistics at a given interval")
	userAgent     = flag.String("useragent", "Fetchbot (https://github.com/PuerkitoBio/fetchbot)", "set the user agent string for the crawler... to be polite")
	crawlDelay    = flag.Duration("crawldelay", 5, "polite crawling delay for the crawler to wait for (second intervals)")
	workerIdleTTL = flag.Duration("workerIdleTTL", 30, "time-to-live for a host url's goroutine (second intervals)")
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

		done := crawl(u.String(), *u)
		if done {
			continue
		}
	}

	fmt.Print("\n=> => Finished.\n\n\n\n")
	fmt.Println(ingestionSet)
	fmt.Println("You caught ", len(badLinks), " bad links.")
	fmt.Println("Total Duplicates: ", len(dup))
	for key := range dup {
		fmt.Println("+++ Duplicate +++ ")
		fmt.Println(key)
	}
}

// parseJSON will parse the local data.json file that is in the same directory as the executable.
// The json file will be a "master" list of links we are going to crawl through.
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

// crawl function that will take a url string and start firing out some crawling functions
// it will return true/false based on the url root it starts with.
func crawl(url string, u url.URL) bool {
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
				// find the bad links in the documents
				badLinks = append(badLinks, ctx.Cmd.URL().String())
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

	// set the fetchbots settings from flag parameters
	f.UserAgent = *userAgent
	f.CrawlDelay = *crawlDelay * time.Second
	f.WorkerIdleTTL = *workerIdleTTL * time.Second

	// First mem stat print must be right after creating the fetchbot
	if *memStats > 0 {
		// Print starting stats
		printMemStats(nil)
		// Run at regular intervals
		runMemStats(f, *memStats)
		// On exit, print ending stats after a GC
		defer func() {
			runtime.GC()
			printMemStats(nil)
		}()
	}

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

	return true
}

// runMemStats controls the debugging and memory allocation statistics
func runMemStats(f *fetchbot.Fetcher, tick time.Duration) {
	var mu sync.Mutex
	var di *fetchbot.DebugInfo

	// Start goroutine to collect fetchbot debug info
	go func() {
		for v := range f.Debug() {
			mu.Lock()
			di = v
			mu.Unlock()
		}
	}()
	// Start ticker goroutine to print mem stats at regular intervals
	go func() {
		c := time.Tick(tick)
		for _ = range c {
			mu.Lock()
			printMemStats(di)
			mu.Unlock()
		}
	}()
}

// printMemStats prints out the memory profile of the application at a given time's state
func printMemStats(di *fetchbot.DebugInfo) {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	buf := bytes.NewBuffer(nil)
	buf.WriteString(strings.Repeat("=", 72) + "\n")
	buf.WriteString("Memory Profile:\n")
	buf.WriteString(fmt.Sprintf("\tAlloc: %d Kb\n", mem.Alloc/1024))
	buf.WriteString(fmt.Sprintf("\tTotalAlloc: %d Kb\n", mem.TotalAlloc/1024))
	buf.WriteString(fmt.Sprintf("\tNumGC: %d\n", mem.NumGC))
	buf.WriteString(fmt.Sprintf("\tGoroutines: %d\n", runtime.NumGoroutine()))
	if di != nil {
		buf.WriteString(fmt.Sprintf("\tNumHosts: %d\n", di.NumHosts))
	}
	buf.WriteString(strings.Repeat("=", 72))
	fmt.Println(buf.String())
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

// scrapeHandler will fire a scraper function on the page if successful response,
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
					ingestionSet.Documents = append(ingestionSet.Documents, response)
					fmt.Println("Total Pages Scraped Successfully: ", len(ingestionSet.Documents))
				}
			} else {
				badLinks = append(badLinks, ctx.Cmd.URL().String())
			}
			// fmt.Printf("[%d] %s %s - %s\n", res.StatusCode, ctx.Cmd.Method(), ctx.Cmd.URL(), res.Header.Get("Content-Type"))
		}
		wrapped.Handle(ctx, res, err)
	})
}

// enqueueLinks will make sure we are adding links to the queue to be processed for crawling/scraping
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

// titleGenerator functino will take in a channel with a pointer to an html.Node
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
	t := strings.TrimSpace(content)
	doc.content = t
	doc.link = url

	return doc
}
