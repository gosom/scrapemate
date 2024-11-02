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
- Headless and Headful support when using JS rendering
- Automatic cookie and session handling
- Rotating HTTP/HTTPS/SOCKS5 proxy support

## Installation

```
go get github.com/gosom/scrapemate
```

## Quickstart


```go
package main

import (
	"context"
	"encoding/csv"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/gosom/scrapemate"
	"github.com/gosom/scrapemate/adapters/writers/csvwriter"
	"github.com/gosom/scrapemate/scrapemateapp"
)

func main() {
	csvWriter := csvwriter.NewCsvWriter(csv.NewWriter(os.Stdout))

	cfg, err := scrapemateapp.NewConfig(
		[]scrapemate.ResultWriter{csvWriter},
	)
	if err != nil {
		panic(err)
	}
	app, err := scrapemateapp.NewScrapeMateApp(cfg)
	if err != nil {
		panic(err)
	}
	seedJobs := []scrapemate.IJob{
		&SimpleCountryJob{
			Job: scrapemate.Job{
				ID:     "identity",
				Method: http.MethodGet,
				URL:    "https://www.scrapethissite.com/pages/simple/",
				Headers: map[string]string{
					"User-Agent": scrapemate.DefaultUserAgent,
				},
				Timeout:    10 * time.Second,
				MaxRetries: 3,
			},
		},
	}
	err = app.Start(context.Background(), seedJobs...)
	if err != nil && err != scrapemate.ErrorExitSignal {
		panic(err)
	}
}

type SimpleCountryJob struct {
	scrapemate.Job
}

func (j *SimpleCountryJob) Process(ctx context.Context, resp *scrapemate.Response) (any, []scrapemate.IJob, error) {
	doc, ok := resp.Document.(*goquery.Document)
	if !ok {
		return nil, nil, fmt.Errorf("failed to cast response document to goquery document")
	}
	var countries []Country
	doc.Find("div.col-md-4.country").Each(func(i int, s *goquery.Selection) {
		var country Country
		country.Name = strings.TrimSpace(s.Find("h3.country-name").Text())
		country.Capital = strings.TrimSpace(s.Find("div.country-info span.country-capital").Text())
		country.Population = strings.TrimSpace(s.Find("div.country-info span.country-population").Text())
		country.Area = strings.TrimSpace(s.Find("div.country-info span.country-area").Text())
		countries = append(countries, country)
	})
	return countries, nil, nil
}

type Country struct {
	Name       string
	Capital    string
	Population string
	Area       string
}

func (c Country) CsvHeaders() []string {
	return []string{"Name", "Capital", "Population", "Area"}
}

func (c Country) CsvRow() []string {
	return []string{c.Name, c.Capital, c.Population, c.Area}
}

```

```
go mod tidy
go run main.go 1>countries.csv
```

(hit CTRL-C to exit)

## Documentation

For the High Level API see this [example](https://github.com/gosom/scrapemate/tree/main/examples/quotes-to-scrape-app).

Read also [how to use high level api](https://blog.gkomninos.com/golang-web-scraping-using-scrapemate)

For the Low Level API see [books.toscrape.com](https://github.com/gosom/scrapemate/tree/main/examples/books-to-scrape-simple)

Additionally, for low level API you can read [the blogpost](https://blog.gkomninos.com/getting-started-with-web-scraping-using-golang-and-scrapemate)


See an example of how you can use `scrapemate` go scrape Google Maps: https://github.com/gosom/google-maps-scraper

## Contributing

Contributions are welcome.

## Licence

Scrapemate is licensed under the MIT License. See LICENCE file

