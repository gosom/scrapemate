package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"

	"github.com/gosom/scrapemate"
	jsfetcher "github.com/gosom/scrapemate/adapters/fetchers/jshttp"
	parser "github.com/gosom/scrapemate/adapters/parsers/goqueryparser"
	provider "github.com/gosom/scrapemate/adapters/providers/memory"
)

const (
	scrollURL = "https://quotes.toscrape.com/scroll"
	apiPath   = "/api/quotes"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	ctx, cancel := context.WithCancelCause(context.Background())
	defer cancel(errors.New("closing example"))

	jobProvider := provider.New()

	httpFetcher, err := jsfetcher.New(jsfetcher.JSFetcherOptions{
		Headless:           true,
		DisableImages:      true,
		PoolSize:           1,
		MaxPagesPerBrowser: 1,
	})
	if err != nil {
		return fmt.Errorf("create js fetcher: %w", err)
	}

	mate, err := scrapemate.New(
		scrapemate.WithContext(ctx, cancel),
		scrapemate.WithJobProvider(jobProvider),
		scrapemate.WithHTTPFetcher(httpFetcher),
		scrapemate.WithHTMLParser(parser.New()),
		scrapemate.WithConcurrency(1),
		scrapemate.WithExitBecauseOfInactivity(2*time.Second),
	)
	if err != nil {
		return fmt.Errorf("create scrapemate: %w", err)
	}
	defer mate.Close()

	resultsDone := make(chan error, 1)
	go func() {
		resultsDone <- writeResults(mate.Results())
	}()

	if err := jobProvider.Push(ctx, newQuoteHooksJob()); err != nil {
		return fmt.Errorf("push seed job: %w", err)
	}

	if err := mate.Start(); err != nil {
		return err
	}

	if err := <-resultsDone; err != nil {
		return fmt.Errorf("write results: %w", err)
	}

	return nil
}

type quoteHooksJob struct {
	scrapemate.Job

	mu        sync.Mutex
	requests  []requestEvent
	responses []responseEvent
}

func newQuoteHooksJob() *quoteHooksJob {
	return &quoteHooksJob{
		Job: scrapemate.Job{
			ID:     "quotes-to-scrape-hooks",
			Method: http.MethodGet,
			URL:    scrollURL,
			Headers: map[string]string{
				"User-Agent": scrapemate.DefaultUserAgent,
			},
			Timeout:    20 * time.Second,
			MaxRetries: 1,
		},
	}
}

func (j *quoteHooksJob) BrowserActions(_ context.Context, page scrapemate.BrowserPage) scrapemate.Response {
	hooks, ok := page.(scrapemate.RequestHookProvider)
	if !ok {
		return scrapemate.Response{
			Error: errors.New("browser page does not support request hooks"),
		}
	}

	hooks.OnRequest(func(url string, headers map[string]string) {
		if !isQuotesAPI(url) {
			return
		}

		j.mu.Lock()
		defer j.mu.Unlock()

		j.requests = append(j.requests, requestEvent{
			URL:     url,
			Headers: pickHeaders(headers, "accept", "user-agent"),
		})
	})

	hooks.OnResponse(func(url string, statusCode int, headers map[string]string) {
		if !isQuotesAPI(url) {
			return
		}

		j.mu.Lock()
		defer j.mu.Unlock()

		j.responses = append(j.responses, responseEvent{
			URL:        url,
			StatusCode: statusCode,
			Headers:    pickHeaders(headers, "content-type", "server"),
		})
	})

	pageResponse, err := page.Goto(j.GetFullURL(), scrapemate.WaitUntilNetworkIdle)
	if err != nil {
		return scrapemate.Response{Error: err}
	}

	if waitErr := page.WaitForSelector(".quote", 10*time.Second); waitErr != nil {
		return scrapemate.Response{Error: waitErr}
	}

	if _, evalErr := page.Eval("window.scrollTo(0, document.body.scrollHeight)"); evalErr != nil {
		return scrapemate.Response{Error: evalErr}
	}

	page.WaitForTimeout(time.Second)

	body, err := page.Content()
	if err != nil {
		return scrapemate.Response{Error: err}
	}

	return scrapemate.Response{
		URL:        pageResponse.URL,
		StatusCode: pageResponse.StatusCode,
		Headers:    pageResponse.Headers,
		Body:       []byte(body),
	}
}

func (j *quoteHooksJob) Process(_ context.Context, resp *scrapemate.Response) (any, []scrapemate.IJob, error) {
	doc, ok := resp.Document.(*goquery.Document)
	if !ok {
		return nil, nil, fmt.Errorf("invalid document type %T expected *goquery.Document", resp.Document)
	}

	j.mu.Lock()
	requests := append([]requestEvent(nil), j.requests...)
	responses := append([]responseEvent(nil), j.responses...)
	j.mu.Unlock()

	return hookResult{
		PageURL:       resp.URL,
		StatusCode:    resp.StatusCode,
		VisibleQuotes: doc.Find(".quote").Length(),
		Requests:      requests,
		Responses:     responses,
	}, nil, nil
}

type hookResult struct {
	PageURL       string          `json:"page_url"`
	StatusCode    int             `json:"status_code"`
	VisibleQuotes int             `json:"visible_quotes"`
	Requests      []requestEvent  `json:"requests"`
	Responses     []responseEvent `json:"responses"`
}

type requestEvent struct {
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
}

type responseEvent struct {
	URL        string            `json:"url"`
	StatusCode int               `json:"status_code"`
	Headers    map[string]string `json:"headers,omitempty"`
}

func isQuotesAPI(rawURL string) bool {
	return strings.Contains(rawURL, apiPath)
}

func pickHeaders(headers map[string]string, names ...string) map[string]string {
	picked := make(map[string]string, len(names))

	for _, name := range names {
		if value := headers[name]; value != "" {
			picked[name] = value
		}
	}

	return picked
}

func writeResults(results <-chan scrapemate.Result) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")

	for result := range results {
		if result.Data == nil {
			continue
		}

		if err := encoder.Encode(result.Data); err != nil {
			return err
		}
	}

	return nil
}
