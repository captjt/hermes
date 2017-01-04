package hermes

import (
	"net/http"
	"strings"
	"sync"

	"github.com/yhat/scrape"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// Scrape function will take a url and fire off pipelines to scrape titles,
// paragraphs, divs and return a Document struct with valid title, content and a link
func Scrape(url string) Document {
	contents := make([]string, 0)

	var docTitle string
	for title := range titleGenerator(rootGenerator(respGenerator(url))) {
		docTitle = title
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

	doc := Document{Title: docTitle, Content: t, Link: url}

	return doc
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
