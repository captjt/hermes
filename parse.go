package main

import (
	"encoding/json"
	"io/ioutil"
	"time"
)

type (
	// Sources struct to model a Type we want to ingest into the elasticsearch index
	// and the links we want to crawl/scrape information to store in our index/type
	Sources struct {
		Type  string
		Links []string
	}

	// Settings struct to model the settings we want to run our hermes application with.
	Settings struct {
		ElasticsearchHost  string        `json:"esHost"`           // host address for the elasticsearch instance
		ElasticsearchIndex string        `json:"esIndex"`          // index name you are going to ingest data into
		CrawlDelay         time.Duration `json:"crawlDelay"`       // delay time for the crawler to abide to
		CancelDuration     time.Duration `json:"cancelDuration"`   // time duration for canceling the crawler (immediate cancel)
		CancelAtURL        string        `json:"cancelUrl"`        // specific URL to cancel the crawler at
		StopDuration       time.Duration `json:"stopDuration"`     // time duration for stopping the crawler (processes links on queue after duration time)
		StopAtURL          string        `json:"stopUrl"`          // specific URL to stop the crawler at for a specific "root"
		MemStatsInterval   time.Duration `json:"memStatsInterval"` // display memory statistics at a given interval
		UserAgent          string        `json:"userAgent"`        // set the user agent string for the crawler... to be polite and identify yourself
		WorkerIdleTTL      time.Duration `json:"workerTimeout"`    // time-to-live for a host URL's goroutine
	}
)

// ParseLinks will parse the local data.json file that is in the same directory as the executable.
// The json file will be a "master" list of links we are going to crawl through.
func ParseLinks() Sources {
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

// ParseSettings will parse a local settings.json file that is in the same directory as the executable.
// The json file will be all the configuration set by the user for the application.
func ParseSettings() Settings {
	var s Settings
	data, errRead := ioutil.ReadFile("./settings.json")
	if errRead != nil {
		panic(errRead)
	}

	errUnmarshal := json.Unmarshal(data, &s)
	if errUnmarshal != nil {
		panic(errUnmarshal)
	}

	return s
}
