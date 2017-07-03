package hermes

import (
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/fetchbot"
	"github.com/PuerkitoBio/goquery"
)

// Scrape function will take a fetchbot.Context struct and a slice of tags to
// try and scrape from the document.
func scrape(ctx *fetchbot.Context, tags []string) (Document, error) {
	document, err := documentResponse(ctx.Cmd.URL())
	if err != nil {
		return Document{}, err
	}

	scrapedDocument := scrapeDocument(ctx, document, tags)
	return scrapedDocument, nil
}

func documentResponse(url *url.URL) (*goquery.Document, error) {
	// http GET request to url's address
	req, err := http.NewRequest("GET", url.String(), nil)
	if err != nil {
		return &goquery.Document{}, err
	}

	// do http GET request to url
	resp, rerr := http.DefaultClient.Do(req)
	if rerr != nil {
		return &goquery.Document{}, rerr
	}

	// generate the goquery Document from io.Reader type
	doc, rrerr := goquery.NewDocumentFromReader(resp.Body)
	if rrerr != nil {
		return &goquery.Document{}, rrerr
	}

	return doc, nil
}

// function to scrape a goquery document and return a structured Document back
func scrapeDocument(ctx *fetchbot.Context, doc *goquery.Document, tags []string) Document {
	var d Document
	var content string

	// scrape page <Title>
	doc.Find("head").Each(func(i int, s *goquery.Selection) {
		title := s.Find("title").Text()
		title = strings.TrimSpace(strings.Replace(title, "\n", " ", -1))
		title = strings.TrimSpace(strings.Replace(title, " ", " ", -1))
		d.Title = title
	})

	// scrape page <Description>
	doc.Find("meta").Each(func(i int, s *goquery.Selection) {
		if name, _ := s.Attr("name"); strings.EqualFold(name, "description") {
			description, _ := s.Attr("content")
			description = strings.TrimSpace(strings.Replace(description, "\n", " ", -1))
			description = strings.TrimSpace(strings.Replace(description, " ", " ", -1))
			d.Description = description
		}
	})

	if len(tags) > 0 {
		for _, tag := range tags {
			text := returnText(doc, tag)
			text = strings.TrimSpace(strings.Replace(text, "\n", " ", -1))
			text = strings.TrimSpace(strings.Replace(text, " ", " ", -1))
			content += " " + text
		}
	} else {
		text := returnText(doc, "default")
		text = strings.TrimSpace(strings.Replace(text, "\n", " ", -1))
		text = strings.TrimSpace(strings.Replace(text, " ", " ", -1))
		content += " " + text
	}

	d.Tag = generateTag(ctx.Cmd.URL().Host)

	d.Content = content
	d.Link = ctx.Cmd.URL().String()
	d.Time = time.Now()

	return d
}

// function to take a custom tag or "default" and return text from that in the goquery document
func returnText(doc *goquery.Document, tag string) string {
	var text string
	doc.Find("body").Each(func(i int, s *goquery.Selection) {
		// default to pulling all the div and p tags else pull custom setting tags
		if tag == "default" {
			text += " " + s.Find("p").Text()
			text += " " + s.Find("div").Text()
		} else {
			text += " " + s.Find(tag).Text()
		}
	})
	return text
}

// generate a tag for a link/document based on the first url string
// (>>sub<< in sub.domain.com or >>domain<< in domain.com)
func generateTag(u string) string {
	var tag string
	s := strings.Split(u, ".")
	if s[0] == "www" && len(s) > 0 {
		tag = s[1]
	} else {
		tag = s[0]
	}
	return tag
}
