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

		documents, done := hermes.Crawl(settings, s, *u)
		if done {
			ingestionSet = documents
		}
	}

	_, err := hermes.Store(hermes.Index{
		Host:      settings.ElasticsearchHost,
		Index:     settings.ElasticsearchIndex,
		Documents: ingestionSet,
	}, settings.ElasticsearchType)
	if err != nil {
		panic(err)
	} else {
		fmt.Println("Successful ETL 🌎🌍🌏")
		os.Exit(0)
	}
}
