package quotes

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/google/uuid"
	"github.com/gosom/scrapemate"
)

// QuoteCollectJob is a job that collects quotes from a page
type QuoteCollectJob struct {
	scrapemate.Job
}

// NewQuoteCollectJob creates a new QuoteCollectJob
func NewQuoteCollectJob(u string) *QuoteCollectJob {
	return &QuoteCollectJob{
		Job: scrapemate.Job{
			// just give it a random id
			ID:     uuid.New().String(),
			Method: http.MethodGet,
			URL:    u,
			Headers: map[string]string{
				"User-Agent": scrapemate.DefaultUserAgent,
			},
			Timeout:    10 * time.Second,
			MaxRetries: 1,
		},
	}
}

// Process is the function that will be called by scrapemate to process the job
func (o *QuoteCollectJob) Process(ctx context.Context, resp *scrapemate.Response) (any, []scrapemate.IJob, error) {
	log := scrapemate.GetLoggerFromContext(ctx)
	log.Info("processing quotes collect job")
	doc, ok := resp.Document.(*goquery.Document)
	if !ok {
		return nil, nil, fmt.Errorf("invalid document type %T expected *goquery.Document", resp.Document)
	}

	if err := CheckLogin(doc); err != nil {
		return nil, nil, err
	}

	quotes, err := parseQuotes(doc)
	if err != nil {
		return nil, nil, err
	}
	var nextJobs []scrapemate.IJob
	nextPage, err := parseNextPage(doc)
	if err == nil {
		nextJobs = append(nextJobs, NewQuoteCollectJob(nextPage))
	}

	return quotes, nextJobs, nil
}

var noNextPage = errors.New("no next page")

func parseNextPage(doc *goquery.Document) (string, error) {
	nextPage := doc.Find(".next > a").AttrOr("href", "")
	if nextPage == "" {
		return "", noNextPage
	}
	nextPage = "http://quotes.toscrape.com" + nextPage
	return nextPage, nil
}
