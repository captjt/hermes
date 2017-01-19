package hermes

import (
	"fmt"
	"net/http"
	"net/url"
	"runtime"
	"sync"
	"time"

	"strings"

	"github.com/PuerkitoBio/fetchbot"
	"github.com/PuerkitoBio/goquery"
)

var (
	mu sync.Mutex // Protect access to dup

	dup = map[string]bool{} // Duplicates table

	settingLinks = map[string]bool{} // Tracking link settings

	ingestionSet []Document // ingestion data TODO make non global

	badLinks []string // bad links TODO make non global
)

// Crawl function that will take a url string and start firing out some crawling functions
// it will return true/false based on the url root it starts with.
func Crawl(settings Settings, linkSettings CustomSettings, u url.URL) ([]Document, bool) {
	// Create the muxer
	mux := fetchbot.NewMux()

	// Handle all errors the same
	mux.HandleErrors(fetchbot.HandlerFunc(func(ctx *fetchbot.Context, res *http.Response, err error) {
		fmt.Printf("[ERR] %s %s - %s\n", ctx.Cmd.Method(), ctx.Cmd.URL(), err)
	}))

	// Handle GET requests for html responses, to parse the body and enqueue all links as HEAD
	// requests.
	mux.Response().Method("GET").ContentType("text/html").Handler(fetchbot.HandlerFunc(
		func(ctx *fetchbot.Context, res *http.Response, err error) {
			// Process the body to find the links
			doc, err := goquery.NewDocumentFromReader(res.Body)
			if err != nil {
				// find the bad links in the documents
				badLinks = append(badLinks, ctx.Cmd.URL().String())
				fmt.Printf("[ERR] %s %s - %s\n", ctx.Cmd.Method(), ctx.Cmd.URL(), err)
				return
			}
			// Enqueue all links as HEAD requests
			enqueueLinks(ctx, doc, u.Host, linkSettings)
		}))

	// Handle HEAD requests for html responses coming from the source host - we don't want
	// to crawl links from other hosts.
	mux.Response().Method("HEAD").Host(u.Host).ContentType("text/html").Handler(fetchbot.HandlerFunc(
		func(ctx *fetchbot.Context, res *http.Response, err error) {
			if _, err := ctx.Q.SendStringGet(ctx.Cmd.URL().String()); err != nil {
				fmt.Printf("[ERR] %s %s - %s\n", ctx.Cmd.Method(), ctx.Cmd.URL(), err)
			}
		}))

	// Create the Fetcher, handle the logging first, then dispatch to the Muxer
	h := scrapeHandler(mux, linkSettings)
	if settings.StopAtURL != "" || settings.CancelAtURL != "" {
		stopURL := settings.StopAtURL
		if settings.CancelAtURL != "" {
			stopURL = settings.CancelAtURL
		}
		h = stopHandler(stopURL, settings.CancelAtURL != "", scrapeHandler(mux, linkSettings))
	}
	f := fetchbot.New(h)

	// set the fetchbots settings from flag parameters
	f.UserAgent = settings.UserAgent
	f.CrawlDelay = settings.CrawlDelay * time.Second
	f.WorkerIdleTTL = 5 * time.Second

	// First mem stat print must be right after creating the fetchbot
	if settings.MemStatsInterval > 0 {
		// Print starting stats
		printMemStats(nil)
		// Run at regular intervals
		runMemStats(f, settings.MemStatsInterval)
		// On exit, print ending stats after a GC
		defer func() {
			runtime.GC()
			printMemStats(nil)
		}()
	}

	// Start processing
	q := f.Start()

	// if a stop or cancel is requested after some duration, launch the goroutine
	// that will stop or cancel.
	if settings.StopDuration*time.Minute > 0 || settings.CancelDuration*time.Minute > 0 {
		after := settings.StopDuration * time.Minute
		stopFunc := q.Close
		if settings.CancelDuration != 0 {
			after = settings.CancelDuration * time.Minute
			stopFunc = q.Cancel
		}

		go func() {
			c := time.After(after)
			<-c
			stopFunc()
		}()
	}

	// Enqueue the seed, which is the first entry in the dup map
	dup[linkSettings.RootLink] = true
	_, err := q.SendStringGet(linkSettings.RootLink)
	if err != nil {
		fmt.Printf("[ERR] GET %s - %s\n", linkSettings.RootLink, err)
	}
	q.Block()

	return ingestionSet, true
}

// stopHandler stops the fetcher if the stopurl is reached. Otherwise it dispatches
// the call to the wrapped Handler.
func stopHandler(stopurl string, cancel bool, wrapped fetchbot.Handler) fetchbot.Handler {
	return fetchbot.HandlerFunc(func(ctx *fetchbot.Context, res *http.Response, err error) {
		if ctx.Cmd.URL().String() == stopurl {
			fmt.Printf(">>>>> STOP URL %s\n", ctx.Cmd.URL())
			// generally not a good idea to stop/block from a handler goroutine
			// so do it in a separate goroutine
			go func() {
				if cancel {
					ctx.Q.Cancel()
				} else {
					ctx.Q.Close()
				}
			}()
			return
		}
		wrapped.Handle(ctx, res, err)
	})
}

// scrapeHandler will fire a scraper function on the page if successful response,
// append the scraped document stored for index ingestion
// and dispatches the call to the wrapped Handler.
func scrapeHandler(wrapped fetchbot.Handler, linkSettings CustomSettings) fetchbot.Handler {
	return fetchbot.HandlerFunc(func(ctx *fetchbot.Context, res *http.Response, err error) {
		if err == nil {
			if res.StatusCode == 200 {
				url := ctx.Cmd.URL().String()
				responseDocument, err := Scrape(url, linkSettings)
				if err != nil {
					fmt.Printf("[ERR] SCRAPE URL: %s - %s", url, err)
				}
				ingestionSet = append(ingestionSet, responseDocument)
			}
			fmt.Printf("[%d] %s %s - %s\n", res.StatusCode, ctx.Cmd.Method(), ctx.Cmd.URL(), res.Header.Get("Content-Type"))
		}
		wrapped.Handle(ctx, res, err)
	})
}

// enqueueLinks will make sure we are adding links to the queue to be processed for crawling/scraping
// this will pull all the href attributes on pages, check for duplicates and add them to the queue
func enqueueLinks(ctx *fetchbot.Context, doc *goquery.Document, host string, settings CustomSettings) {
	mu.Lock()
	doc.Find("a[href]").Each(func(i int, s *goquery.Selection) {
		val, _ := s.Attr("href")
		// Resolve address
		u, err := ctx.Cmd.URL().Parse(val)
		if err != nil {
			fmt.Printf("error: resolve URL %s - %s\n", val, err)
			return
		}

		// catch the duplicate urls here before trying to add them to the queue
		if !dup[u.String()] {

			// tld & subdomain
			if settings.TopLevelDomain == true && settings.Subdomain == true {
				rootDomain := getDomain(host)
				current := getDomain(u.Host)

				if rootDomain == current {
					err := addLink(ctx, u)
					if err != nil {
						fmt.Printf("error: enqueue head %s - %s\n", u, err)
						return
					}
				} else {
					fmt.Printf("error: out of domain scope -- %s != %s\n", u.Host, host)
					return
				}
			}

			// tld check
			if settings.TopLevelDomain == true {
				rootTLD := getTLD(host)
				current := getTLD(u.Host)

				if rootTLD == current {
					err := addLink(ctx, u)
					if err != nil {
						fmt.Printf("error: enqueue head %s - %s\n", u, err)
						return
					}
				}
			}

			// subdomain check
			if settings.Subdomain == true {
				rootDomain := getDomain(host)
				current := getDomain(u.Host)

				if rootDomain == current {
					err := addLink(ctx, u)
					if err != nil {
						fmt.Printf("error: enqueue head %s - %s\n", u, err)
						return
					}
				} else {
					fmt.Printf("error: out of domain scope -- %s != %s\n", u.Host, host)
					return
				}
			}
		}

		return
	})
	mu.Unlock()
}

// addLink will add a url to fetchbot's queue and to the global hashmap to audit for duplicates
func addLink(ctx *fetchbot.Context, u *url.URL) error {
	if _, err := ctx.Q.SendStringHead(u.String()); err != nil {
		return err
	}
	dup[u.String()] = true
	return nil
}

// getDomain will parse a url and return the domain with the tld on it (ie. example.com)
func getDomain(u string) (root string) {
	s := strings.Split(u, ".")
	last := len(s) - 1
	runnerUp := last - 1
	root = s[runnerUp] + "." + s[last]
	return
}

// getTLD will parse a url type and return the top-level domain (.com, .edu, .gov, etc.)
func getTLD(u string) (tld string) {
	s := strings.Split(u, ".")
	last := len(s) - 1
	tld = s[last]
	return
}
