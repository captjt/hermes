package hermes

import (
	"net/http"
	"strings"
	"sync"

	"errors"

	"github.com/PuerkitoBio/fetchbot"
	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

// Scrape function will take a url and fire off pipelines to scrape titles,
// paragraphs, divs and return a Document struct with valid title, content and a link
func Scrape(ctx *fetchbot.Context, cs CustomSettings) (Document, error) {
	document := Document{}
	for document = range documentGenerator(rootGenerator(respGenerator(ctx.Cmd.URL().String())), ctx, cs) {
		return document, nil
	}
	return document, errors.New("Scraping error")
}

// function to generate a response from a url pass into it
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

// function to generate an html node with an http.Response pointer passed into it
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

// documentGenerator function will take in a channel with a pointer to an html.Node
// type and customized settings and it will fire off scraping mechanisms to return a Document
func documentGenerator(in <-chan *html.Node, ctx *fetchbot.Context, cs CustomSettings) <-chan Document {
	var wg sync.WaitGroup
	out := make(chan Document)
	for root := range in {
		wg.Add(1)
		go func(root *html.Node) {
			doc := goquery.NewDocumentFromNode(root)
			out <- scrapeDocument(ctx, doc, cs.Tags)
			wg.Done()
		}(root)
	}
	go func() {
		wg.Wait()
		close(out)
	}()
	return out
}

// function to scrape a goquery document and return a structured Document back
func scrapeDocument(ctx *fetchbot.Context, doc *goquery.Document, tags []string) Document {
	var (
		d       Document
		content string
	)
	// scrape page <Title>
	doc.Find("head").Each(func(i int, s *goquery.Selection) {
		title := s.Find("title").Text()
		d.Title = strings.TrimSpace(strings.Replace(title, " ", " ", -1))
	})

	// scrape page <Description>
	doc.Find("meta").Each(func(i int, s *goquery.Selection) {
		if name, _ := s.Attr("name"); strings.EqualFold(name, "description") {
			description, _ := s.Attr("content")
			d.Description = strings.TrimSpace(strings.Replace(description, " ", " ", -1))
		}
	})

	if len(tags) > 0 {
		for _, tag := range tags {
			text := returnText(doc, tag)
			content += " " + strings.TrimSpace(strings.Replace(text, " ", " ", -1))
		}
	} else {
		text := returnText(doc, "default")
		content += " " + strings.TrimSpace(strings.Replace(text, " ", " ", -1))
	}

	d.Tag = generateTag(ctx.Cmd.URL().Host)

	d.Content = content
	d.Link = ctx.Cmd.URL().String()

	return d
}

// function to take a custom tag or "default" and return text from that in the goquery document
func returnText(doc *goquery.Document, tag string) (text string) {
	doc.Find("body").Each(func(i int, s *goquery.Selection) {
		// default to pulling all the div and p tags else pull custom setting tags
		if tag == "default" {
			text += " " + s.Find("p").Text()
			text += " " + s.Find("div").Text()
		} else {
			text += " " + s.Find(tag).Text()
		}
	})
	return
}

// generate a tag for a link/document based on the first url string
// (>>sub<< in sub.domain.com or >>domain<< in domain.com)
func generateTag(u string) (tag string) {
	s := strings.Split(u, ".")
	if s[0] == "www" && len(s) > 0 {
		tag = s[1]
	} else {
		tag = s[0]
	}
	return
}
