package hermes

import (
	"encoding/json"
	"io/ioutil"
	"time"

	log "github.com/Sirupsen/logrus"
)

type (
	// CustomSettings struct to model custom settings we want to scrape from a specific page
	CustomSettings struct {
		RootLink       string   `json:"link"`
		Tags           []string `json:"tags"`
		Subdomain      bool     `json:"subdomain"`
		TopLevelDomain bool     `json:"top_level_domain"`
	}

	// Sources struct to model a Type we want to ingest into the elasticsearch index
	// and the links we want to crawl/scrape information to store in our index/type
	Sources struct {
		Links []CustomSettings `json:"links"` // an array of all the URL strings we want to start our crawler at
	}

	// Settings struct to model the settings we want to run our hermes application with.
	Settings struct {
		ElasticsearchHost  string        `json:"es_host"`            // host address for the elasticsearch instance
		ElasticsearchIndex string        `json:"es_index"`           // index name you are going to ingest data into
		ElasticsearchType  string        `json:"es_type"`            // type name you are going to ingest data into
		CrawlDelay         time.Duration `json:"crawl_delay"`        // delay time for the crawler to abide to
		CancelDuration     time.Duration `json:"cancel_duration"`    // time duration for canceling the crawler (immediate cancel)
		CancelAtURL        string        `json:"cancel_url"`         // specific URL to cancel the crawler at
		StopDuration       time.Duration `json:"stop_duration"`      // time duration for stopping the crawler (processes links on queue after duration time)
		StopAtURL          string        `json:"stop_url"`           // specific URL to stop the crawler at for a specific "root"
		MemStatsInterval   time.Duration `json:"mem_stats_interval"` // display memory statistics at a given interval
		UserAgent          string        `json:"user_agent"`         // set the user agent string for the crawler... to be polite and identify yourself
		WorkerIdleTTL      time.Duration `json:"worker_timeout"`     // time-to-live for a host URL's goroutine
		AutoClose          bool          `json:"autoclose"`          // sets the application to terminate if the WorkerIdleTTL time is passed (must be true)
		EnableLogging      bool          `json:"enable_logging"`     // sets whether or not to log to a file
	}
)

// ParseLinks will parse the local data.json file that is in the same directory as the executable.
// The json file will be a "master" list of links we are going to crawl through.
func ParseLinks() Sources {
	var s Sources
	data, errRead := ioutil.ReadFile("./data.json")
	if errRead != nil {
		log.WithFields(log.Fields{
			"error": errRead,
		}).Panic("an error reading data.json file")
		panic(errRead)
	}

	errUnmarshal := json.Unmarshal(data, &s)
	if errUnmarshal != nil {
		log.WithFields(log.Fields{
			"error": errUnmarshal,
		}).Panic("an error unmarshaling data.json file")
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
		log.WithFields(log.Fields{
			"error": errRead,
		}).Panic("an error reading settings.json file")
		panic(errRead)
	}

	errUnmarshal := json.Unmarshal(data, &s)
	if errUnmarshal != nil {
		log.WithFields(log.Fields{
			"error": errUnmarshal,
		}).Panic("an error unmarshaling settings.json file")
		panic(errUnmarshal)
	}

	return s
}
