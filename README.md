# [Hermes](https://en.wikipedia.org/wiki/Hermes) 🏃💨

![Boom](./docs/static_files/power-to-the-masses.png)

This is a combination of a couple awesome packages [scrape](https://github.com/yhat/scrape) and [fetchbot](https://github.com/PuerkitoBio/fetchbot) that will crawl a list of links and scrape the pages.

The premise behind all of this is that I wanted to have sort of an all in one way to crawl through sites and scrape it's content to store into an Elasticsearch index.

The next few things I will need to do is ...
* add in some ingestion features for the Documents (scraped pages) to be uploaded to a configurable Elasticsearch instance
    * will start with Elasticsearch V. 2.0 - then -> support later versioning
* clean up the code to make it more reusable and modular
* create a solid way for people to incorporate these features into their own applications
