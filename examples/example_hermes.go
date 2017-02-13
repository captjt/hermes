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
