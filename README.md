# scrapemate
[![Documentation](https://img.shields.io/badge/Documentation-Read%20Here-blue)](https://gosom.github.io/scrapemate)
[![GoDoc](https://godoc.org/github.com/gosom/scrapemate?status.svg)](https://godoc.org/github.com/gosom/scrapemate)
![build](https://github.com/gosom/scrapemate/actions/workflows/build.yml/badge.svg)
[![Go Report Card](https://goreportcard.com/badge/github.com/gosom/scrapemate)](https://goreportcard.com/report/github.com/gosom/scrapemate)

[Scrapemate](https://gosom.github.io/scrapemate) is a web crawling and scraping framework written in Golang. It is designed to be simple and easy to use, yet powerful enough to handle complex scraping tasks.


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

## Browser Engines

Scrapemate supports two browser engines for JavaScript rendering: **Playwright** (default) and **Rod**. The browser engine is selected at compile time using Go build tags.

### Playwright (Default)

Playwright is used by default when no build tags are specified. It requires the Playwright browsers to be installed.

```bash
# Install playwright browsers
go run github.com/playwright-community/playwright-go/cmd/playwright install --with-deps chromium
```

Build and run without any special tags:

```bash
go build ./...
```

### Rod

Rod is a pure Go solution that uses the Chrome DevTools Protocol directly. To use Rod instead of Playwright, compile with the `rod` build tag.

```bash
# Build with Rod support
go build -tags rod ./...

# Run with Rod support
go run -tags rod ./...
```

Rod will automatically download and manage Chrome/Chromium if it's not already available on your system.

### Choosing Between Playwright and Rod

| Feature | Playwright | Rod |
|---------|-----------|-----|
| **Dependencies** | Requires browser installation step | Auto-downloads browser |
| **Browser Support** | Chromium, Firefox, WebKit | Chromium only |
| **Performance** | Slightly higher overhead | Lower overhead, pure Go |
| **Docker** | Larger image size | Smaller image size |
| **API Stability** | Very stable | Stable |

**Recommendation**: Use Playwright if you need multi-browser support or are already familiar with Playwright. Use Rod if you prefer a pure Go solution with automatic browser management and smaller Docker images.

### Example Usage

The [books-to-scrape-simple](https://github.com/gosom/scrapemate/tree/main/examples/books-to-scrape-simple) example demonstrates how to use both browser engines:

```bash
# Run with Playwright (default)
# First install browsers: go run github.com/playwright-community/playwright-go/cmd/playwright install --with-deps chromium
go run . -js

# Run with Rod
go run -tags rod . -js

# Run with Rod in stealth mode
go run -tags rod . -js -stealth
```


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

## Migrating from v0.9.x to v1.0.0

Version 1.0.0 introduces a `BrowserPage` interface abstraction to support multiple browser engines. This is a breaking change for users who use JavaScript rendering with `BrowserActions`.

### Update `BrowserActions` signature

```go
// Before (v0.9.x)
func (j *MyJob) BrowserActions(ctx context.Context, page playwright.Page) scrapemate.Response {
    page.Goto("https://example.com", playwright.PageGotoOptions{
        WaitUntil: playwright.WaitUntilStateNetworkidle,
    })
    html, _ := page.Content()
    return scrapemate.Response{Body: []byte(html)}
}

// After (v1.0.0)
func (j *MyJob) BrowserActions(ctx context.Context, page scrapemate.BrowserPage) scrapemate.Response {
    resp, err := page.Goto("https://example.com", scrapemate.WaitUntilNetworkIdle)
    if err != nil {
        return scrapemate.Response{Error: err}
    }
    return scrapemate.Response{
        Body:       resp.Body,
        StatusCode: resp.StatusCode,
    }
}
```

### Accessing the underlying browser page

If you need browser-specific features, use `Unwrap()`:

```go
// For Playwright
pwPage := page.Unwrap().(playwright.Page)

// For Rod (when compiled with -tags rod)
rodPage := page.Unwrap().(*rod.Page)
```

See [CHANGELOG.md](CHANGELOG.md) for the full list of changes.

## Documentation

You can find more documentation [here](https://gosom.github.io/scrapemate)

For the High Level API see this [example](https://github.com/gosom/scrapemate/tree/main/examples/quotes-to-scrape-app).

Read also [how to use high level api](https://blog.gkomninos.com/golang-web-scraping-using-scrapemate)

For the Low Level API see [books.toscrape.com](https://github.com/gosom/scrapemate/tree/main/examples/books-to-scrape-simple)

Additionally, for low level API you can read [the blogpost](https://blog.gkomninos.com/getting-started-with-web-scraping-using-golang-and-scrapemate)


See an example of how you can use `scrapemate` go scrape Google Maps: https://github.com/gosom/google-maps-scraper

## Contributing

Contributions are welcome.

## Licence

Scrapemate is licensed under the MIT License. See LICENCE file

