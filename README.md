# Whats is [Hermes](https://en.wikipedia.org/wiki/Hermes)? 🏃💨

This is a combination of a couple awesome packages [goquery](https://github.com/PuerkitoBio/goquery) + [fetchbot](https://github.com/PuerkitoBio/fetchbot) that will crawl a list of links and scrape the pages.

This package is completely a _proof-of-concept_ idea to use. The storage layer only interacts with Elasticsearch at the moment.

[As of 4-28-2017]: Will be working on refactoring this full package. Will be a more idiomatic version. This was something initially to learn more about Go and web crawling/scraping.

I will add more examples of how to use the newer refactor as well.

---

![Hermes](https://github.com/jtaylor32/hermes/blob/master/docs/static_files/power-to-the-masses.png "Hermes Logo")

## Install

`go get github.com/jtaylor32/hermes`

## API Usage

### Runner

Basically a **Runner** is just an easier way to configure a web crawler combined with a scraper. Depending on your *TopLevelDomain* + *Subdomain* flags it will run through all of the nested links starting at the *URL*. The other struct fields will make your Runner more granular as well. The *Tags* are specific HTML tags you would like to pull from pages you are scraping.

A call to `Runner.Crawl()` will start you Runner and return an array of **Documents** and *error*. It will handle all the dynamic scraping and running under the scenes based on your Runner fields/values.

### Elasticsearch

**Elasticsearch** is a struct of an Elasticsearch *host, index, and type*. This is where you can specify where you are storing the Documents from the `Crawl()`.

## License

The [BSD 3-Clause license](http://opensource.org/licenses/BSD-3-Clause), the same as the [Go language](http://golang.org/LICENSE).

## Acknowledgments

Huge thanks to Martin Angers [@mna](https://github.com/mna) and the work he has done on all his projects!
