package main

import (
	"context"
	"encoding/csv"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"

	"github.com/gosom/scrapemate"
	jsfetcher "github.com/gosom/scrapemate/adapters/fetchers/jshttp"
	fetcher "github.com/gosom/scrapemate/adapters/fetchers/nethttp"
	parser "github.com/gosom/scrapemate/adapters/parsers/goqueryparser"
	provider "github.com/gosom/scrapemate/adapters/providers/memory"

	"booktoscrapesimple/bookstoscrape"
)

func main() {
	if err := run(); err != nil {
		if err == scrapemate.ErrorExitSignal {
			os.Exit(0)
		}
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	os.Exit(0)
}

func run() error {
	ctx, cancel := context.WithCancelCause(context.Background())
	defer cancel(errors.New("deferred cancel"))

	var useJS bool
	flag.BoolVar(&useJS, "js", false, "use javascript")
	flag.Parse()
	// create a new memory provider
	provider := provider.New()
	// we will start  a go routine that will push jobs to the provider
	// here we need to extract all books from https://books.toscrape.com/
	// In this case we just need to push the initial collect job
	go func() {
		job := &bookstoscrape.BookCollectJob{
			Job: scrapemate.Job{
				// just give it a random id
				ID:     uuid.New().String(),
				Method: http.MethodGet,
				URL:    "https://books.toscrape.com/",
				Headers: map[string]string{
					"User-Agent": scrapemate.DefaultUserAgent,
				},
				Timeout:    10 * time.Second,
				MaxRetries: 3,
			},
		}
		provider.Push(ctx, job)
	}()

	var httpFetcher scrapemate.HttpFetcher
	var err error
	switch useJS {
	case true:
		httpFetcher, err = jsfetcher.New(true)
		if err != nil {
			return err
		}
	default:
		httpFetcher = fetcher.New(&http.Client{
			Timeout: 10 * time.Second,
		})
	}

	mate, err := scrapemate.New(
		scrapemate.WithContext(ctx, cancel),
		scrapemate.WithJobProvider(provider),
		scrapemate.WithHttpFetcher(httpFetcher),
		scrapemate.WithConcurrency(10),
		scrapemate.WithHtmlParser(parser.New()),
	)

	if err != nil {
		return err
	}

	// process the results here
	resultsDone := make(chan struct{})
	go func() {
		defer close(resultsDone)
		if err := writeCsv(mate.Results()); err != nil {
			cancel(err)
			return
		}
	}()

	err = mate.Start()
	<-resultsDone
	return err
}

func writeCsv(results <-chan scrapemate.Result) error {
	w := csv.NewWriter(os.Stdout)
	defer w.Flush()
	headersWritten := false
	screenshotFolder := "screenshots"
	if err := os.MkdirAll(screenshotFolder, 0755); err != nil {
		return err
	}
	for result := range results {
		if result.Data == nil {
			continue
		}
		product, ok := result.Data.(bookstoscrape.Product)
		if !ok {
			return fmt.Errorf("unexpected data type: %T", result.Data)
		}
		if result.Job.DoScreenshot() && len(product.Screenshot) > 0 {
			path := fmt.Sprintf("%s/%s.png", screenshotFolder, product.UPC)
			if err := os.WriteFile(path, product.Screenshot, 0644); err != nil {
				return err
			}
		}
		if !headersWritten {
			if err := w.Write(product.CsvHeaders()); err != nil {
				return err
			}
			headersWritten = true
		}
		if err := w.Write(product.CsvRow()); err != nil {
			return err
		}
		w.Flush()
	}
	return w.Error()
}
