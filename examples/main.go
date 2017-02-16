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
