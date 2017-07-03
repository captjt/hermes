package main

import (
	"fmt"
	"log"
	"net/url"
	"os"

	"github.com/jtaylor32/hermes"
)

func main() {
	// Parse the seed URL string
	u, e := url.Parse("http://jt.codes")
	if e != nil {
		log.Fatal(e)
	}

	r := hermes.New()

	// override custom fields on the new Runner
	r.URL = u
	r.Tags = []string{"div", "h1", "p"}

	// Start the Runner
	i, b := r.Crawl()
	if b != nil {
		log.Fatal(b)
	}

	// Elasticsearch settings
	es := hermes.Elasticsearch{
		Host:  "http://localhost:9200",
		Index: "hermes_index",
		Type:  "hermes_type",
	}

	// Start the storage ingest
	in := es.Store(len(i), i)
	if in != nil {
		log.Fatal(e)
	}

	fmt.Println("[ ✓ ] 🏃💨")
	os.Exit(0)
}
