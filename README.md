<img src="https://github.com/jtaylor32/hermes/blob/master/docs/static_files/power-to-the-masses.png" alt="Boom Hermes" align="right" />

Whats is [Hermes](https://en.wikipedia.org/wiki/Hermes)? 🏃💨
====================
This is a combination of a couple awesome packages [goquery](https://github.com/PuerkitoBio/goquery) + [fetchbot](https://github.com/PuerkitoBio/fetchbot) that will crawl a list of links and scrape the pages.

The premise behind all of this is that I wanted to have sort of an all in one way to crawl through sites and scrape it's content to store into an Elasticsearch index.

This is a completely fun prototype.  I do plan on abstracting it out eventually and making it a reusable package but for now I am just making it something to kind of simulate a simple ETL of webpage content.

Install
====================

`go get github.com/jtaylor32/hermes`

Example
====================

**You will need to make sure you follow the example** `data.json` **and** `settings.json` **files**

```go
package main

import (
	"fmt"
	"log"
	"net/url"
	"os"

	"github.com/jtaylor32/hermes"
)

func main() {
	// create an array of Documents
	var ingestionSet []hermes.Document
	// parse the data.json with type/links to pass into the crawler
	src := hermes.ParseLinks()

	// parse the settings.json with settings to pass into hermes
	settings := hermes.ParseSettings()

	// start the crawler
	for _, s := range src.Links {
		u, err := url.Parse(s.RootLink)
		if err != nil {
			log.Fatal(err)
		}

		documents, done := hermes.Crawl(settings, s, u)
		if done {
			ingestionSet = documents
		}

		fmt.Println("Total Documents in ingestion set: ", len(ingestionSet))

		e := hermes.Store(
			len(ingestionSet),
			settings.ElasticsearchHost,
			settings.ElasticsearchIndex,
			settings.ElasticsearchType,
			ingestionSet,
		)
		if e != nil {
			log.Fatal(e)
		}
	}

	fmt.Println("Successful ETL 🌎🌍🌏")
	os.Exit(0)
}
```

Acknowledgments
====================

Huge thanks to [PuerkitoBio](https://github.com/PuerkitoBio) and the work he has done on all his projects!
