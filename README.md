<img src="https://github.com/jtaylor32/hermes/blob/master/docs/static_files/power-to-the-masses.png" alt="Boom Hermes" align="right" />

Whats is [Hermes](https://en.wikipedia.org/wiki/Hermes)? 🏃💨
====================
This is a combination of a couple awesome packages [goquery](https://github.com/PuerkitoBio/goquery) + [fetchbot](https://github.com/PuerkitoBio/fetchbot) that will crawl a list of links and scrape the pages.

The premise behind all of this is that I wanted to have sort of an all in one way to crawl through sites and scrape it's content to store into an Elasticsearch index.

This is a completely fun prototype.  I do plan on abstracting it out eventually and making it a reusable package but for now I am just making it something to kind of simulate a simple ETL of webpage content.

Install
=======

`go get github.com/jtaylor32/hermes`

Example
=======

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

	u, e := url.Parse("http://jt.codes")
	if e != nil {
		panic(e)
	}

	r := hermes.Runner{
		CrawlDelay:       1,
		CancelDuration:   60,
		CancelAtURL:      "",
		StopDuration:     60,
		StopAtURL:        "",
		MemStatsInterval: 0,
		UserAgent:        "(Hermes Bot)",
		WorkerIdleTTL:    10,
		AutoClose:        true,
		URL:              u,
		Tags:             []string{"div", "h1", "p"},
		TopLevelDomain:   true,
		Subdomain:        true,
	}

	i, b := r.Crawl()
	if b {
		ingestionSet = i
	}

	fmt.Println("Total Documents in ingestion set: ", len(ingestionSet))

	es := hermes.Elasticsearch{
		Host:  "http://localhost:9200",
		Index: "search_index",
		Type:  "feb_16",
	}

	in := es.Store(ingestionSet)
	if in != nil {
		log.Fatal(e)
	}

	fmt.Println("Successful ETL 🌎🌍🌏")
	os.Exit(0)
}

```

License
=======

The [BSD 3-Clause license](http://opensource.org/licenses/BSD-3-Clause), the same as the [Go language](http://golang.org/LICENSE).

Acknowledgments
===============

Huge thanks to [PuerkitoBio](https://github.com/PuerkitoBio) and the work he has done on all his projects!
