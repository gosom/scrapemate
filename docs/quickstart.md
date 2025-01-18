## Simple scraper

The following example will extract all the quotes from http://quotes.toscrape.com/
in a CSV file.

```sh
mkdir scrapequotes
cd scrapequotes
go mod init scrapequotes
touch main.go
```

Paste the contents below to `main.go`.


```go
package main

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/gosom/scrapemate"
	"github.com/gosom/scrapemate/adapters/writers/csvwriter"
	"github.com/gosom/scrapemate/scrapemateapp"

	"github.com/google/uuid"
)

func main() {
	if err := run(); err != nil && !errors.Is(err, context.Canceled) {
		os.Stderr.WriteString(err.Error() + "\n")

		os.Exit(1)
	}

	os.Exit(0)
}

func run() error {
	// Create a context with a cancel function
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// when the user sends a SIGINT signal, cancel the context
	gracefulShutdown := make(chan os.Signal, 1)
	signal.Notify(gracefulShutdown, os.Interrupt)

	go func() {
		<-gracefulShutdown

		cancel()
	}()

	// Create a new csv writer, this will write the results to stdout in a csv format
	csvWriter := csvwriter.NewCsvWriter(csv.NewWriter(os.Stdout))
	writers := []scrapemate.ResultWriter{
		csvWriter,
	}

	// Create a new scrapemate app
	cfg, err := scrapemateapp.NewConfig(
		writers,
		scrapemateapp.WithExitOnInactivity(2*time.Second),
	)
	if err != nil {
		return err
	}

	app, err := scrapemateapp.NewScrapeMateApp(cfg)
	if err != nil {
		return err
	}

	// Start the app with the seed jobs
	seedJobs := []scrapemate.IJob{
		newQuoteCollectJob("https://quotes.toscrape.com/"),
	}

	return app.Start(ctx, seedJobs...)
}

// quoteCollectJob is a job that will collect quotes from the quotes.toscrape.com website
type quoteCollectJob struct {
	scrapemate.Job
}

// newQuoteCollectJob creates a new quoteCollectJob
// u is the url to collect quotes from
func newQuoteCollectJob(u string) *quoteCollectJob {
	return &quoteCollectJob{
		Job: scrapemate.Job{
			ID:     uuid.New().String(),
			Method: http.MethodGet,
			URL:    u,
			Headers: map[string]string{
				"User-Agent": scrapemate.DefaultUserAgent,
			},
			Timeout:    5 * time.Second,
			MaxRetries: 2,
		},
	}
}

// Process is the function that will be called by scrapemate to process the job
// ctx is the context of the Job
// resp contains the response after the job's request has been made and we have a response
// returns the data that will be written to the writers, the next jobs to be processed and an error if any
func (o *quoteCollectJob) Process(ctx context.Context, resp *scrapemate.Response) (any, []scrapemate.IJob, error) {
	log := scrapemate.GetLoggerFromContext(ctx)
	log.Info("processing quotes collect job")

	doc, ok := resp.Document.(*goquery.Document)
	if !ok {
		return nil, nil, fmt.Errorf("invalid document type %T expected *goquery.Document", resp.Document)
	}

	quotes, err := parseQuotes(doc)
	if err != nil {
		return nil, nil, err
	}
	var nextJobs []scrapemate.IJob

	nextPage, err := parseNextPage(doc)
	if err == nil {
		nextJobs = append(nextJobs, newQuoteCollectJob(nextPage))
	}

	return quotes, nextJobs, nil
}

// Quote represents a quote from the quotes.toscrape.com website
type Quote struct {
	Author string
	Text   string
	Tags   []string
}

// CsvHeaders returns the headers for the csv
func (q Quote) CsvHeaders() []string {
	return []string{"author", "text", "tags"}
}

// CsvRow returns the csv row for the quote
func (q Quote) CsvRow() []string {
	return []string{q.Author, q.Text, strings.Join(q.Tags, ",")}
}

// parseQuotes parses the quotes from the document
func parseQuotes(doc *goquery.Document) ([]Quote, error) {
	var quotes []Quote

	doc.Find(".quote").Each(func(i int, s *goquery.Selection) {
		quote := Quote{
			Author: s.Find(".author").Text(),
			Text:   s.Find(".text").Text(),
		}

		s.Find(".tag").Each(func(i int, s *goquery.Selection) {
			if s.Text() != "" {
				quote.Tags = append(quote.Tags, s.Text())
			}
		})

		quotes = append(quotes, quote)
	})

	return quotes, nil
}

var ErrNoNextPage = errors.New("no next page")

// parseNextPage parses the next page from the document
// returns the next page url or an error if there is no next page
func parseNextPage(doc *goquery.Document) (string, error) {
	nextPage := doc.Find(".next > a").AttrOr("href", "")
	if nextPage == "" {
		return "", ErrNoNextPage
	}

	nextPage = "http://quotes.toscrape.com" + nextPage

	return nextPage, nil
}
```

Then run:

```sh
go mod download
```

Run the scraper:

```sh
go run main.go > quotes.csv
```

you will see the log output in the console.

Then the results will be `quotes.csv` file

