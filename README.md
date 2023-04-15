# scrapemate
[![GoDoc](https://godoc.org/github.com/gosom/scrapemate?status.svg)](https://godoc.org/github.com/gosom/scrapemate)
![build](https://github.com/gosom/scrapemate/actions/workflows/build.yml/badge.svg)
[![Go Report Card](https://goreportcard.com/badge/github.com/gosom/scrapemate)](https://goreportcard.com/report/github.com/gosom/scrapemate)

Scrapemate is a web crawling and scraping framework written in Golang. It is designed to be simple and easy to use, yet powerful enough to handle complex scraping tasks.


## Features

- Low level API & Easy High Level API
- Customizable retry and error handling
- Javascript Rendering with ability to control the browser
- Screenshots support (when JS rendering is enabled)
- Capability to write your own result exporter
- Capability to write results in multiple sinks
- Default CSV writer
- Caching (File/LevelDB/Custom)
- Custom job providers (memory provider included)

## Usage

For the High Level API see this [example](https://github.com/gosom/scrapemate/tree/main/examples/quotes-to-scrape-app).

Read also (how to use high level api)[https://blog.gkomninos.com/golang-web-scraping-using-scrapemate]

For the Low Level API see [books.toscrape.com](https://github.com/gosom/scrapemate/tree/main/examples/books-to-scrape-simple)

Additionally, for low level API you can read [the blogpost](https://blog.gkomninos.com/getting-started-with-web-scraping-using-golang-and-scrapemate)

## Contributing

Contributions are welcome.

## Licence

Scrapemate is licensed under the MIT License. See LICENCE file

