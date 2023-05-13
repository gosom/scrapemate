package bookstoscrape

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/google/uuid"
	"github.com/gosom/scrapemate"
)

type BookCollectJob struct {
	scrapemate.Job
}

func (o *BookCollectJob) Process(ctx context.Context, resp *scrapemate.Response) (any, []scrapemate.IJob, error) {
	log := scrapemate.GetLoggerFromContext(ctx)
	log.Info("processing book collect job")
	doc, ok := resp.Document.(*goquery.Document)
	if !ok {
		return nil, nil, fmt.Errorf("invalid document type %T expected *goquery.Document", resp.Document)
	}
	var nextJobs []scrapemate.IJob
	var productLinks []string
	doc.Find("article.product_pod >div.image_container>a").Each(func(i int, s *goquery.Selection) {
		href, _ := s.Attr("href")
		productLinks = append(productLinks, href)
	})
	for _, link := range productLinks {
		if !strings.HasPrefix(link, "catalogue") {
			link = "catalogue/" + link
		}
		nextJobs = append(nextJobs, &BookDetailJob{
			Job: scrapemate.Job{
				ID:     uuid.New().String(),
				Method: http.MethodGet,
				URL:    "https://books.toscrape.com/" + link,
				Headers: map[string]string{
					"User-Agent": scrapemate.DefaultUserAgent,
				},
				Timeout:        10 * time.Second,
				MaxRetries:     3,
				Priority:       1,
				TakeScreenshot: true,
			},
		})
	}
	nextLink := doc.Find("li.next>a").AttrOr("href", "")
	if nextLink != "" {
		if !strings.HasPrefix(nextLink, "catalogue") {
			nextLink = "catalogue/" + nextLink
		}
		nextLink = "http://books.toscrape.com/" + nextLink
		nextJobs = append(nextJobs, &BookCollectJob{
			Job: scrapemate.Job{
				ID:     uuid.New().String(),
				Method: http.MethodGet,
				URL:    nextLink,
				Headers: map[string]string{
					"User-Agent": scrapemate.DefaultUserAgent,
				},
				Timeout:    10 * time.Second,
				MaxRetries: 3,
			},
		})
	}

	return nil, nextJobs, nil
}
