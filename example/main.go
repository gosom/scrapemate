package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/google/uuid"
	"github.com/gosom/kit/logging"
	"github.com/gosom/scrapemate/providers"

	"github.com/gosom/scrapemate"
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

type BookCollectJob struct {
	scrapemate.Job
}

func (o *BookCollectJob) Process(ctx context.Context, w scrapemate.JobWriter) ([]scrapemate.IJob, error) {
	log := ctx.Value("log").(logging.Logger)
	log.Info("processing book collect job")
	doc := o.Response.Document
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
				Timeout:    10 * time.Second,
				MaxRetries: 3,
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

	return nextJobs, nil
}

type BookDetailJob struct {
	scrapemate.Job
}

func (o *BookDetailJob) Process(ctx context.Context, w scrapemate.JobWriter) ([]scrapemate.IJob, error) {
	log := ctx.Value("log").(logging.Logger)
	log.Info("processing book detail job")
	doc := o.Response.Document
	title := doc.Find("div.product_main>h1").Text()
	fmt.Println(title)
	return nil, nil
}

type Product struct {
	UPC                string
	ProductType        string
	Currency           string
	PriceExclTax       float64
	PriceInclTax       float64
	Tax                float64
	InStock            bool
	Availability       int
	NumbersOfReviews   int
	StarRating         float64
	ProductDescription string
}

func run() error {
	ctx, cancel := context.WithCancelCause(context.Background())
	defer cancel(errors.New("deferred cancel"))
	// create a new memory provider
	provider := providers.NewMemoryProvider()
	// we will start  a go routine that will push jobs to the provider
	// here we need to extract all books from https://books.toscrape.com/
	// In this case we just need to push the initial collect job
	go func() {
		job := &BookCollectJob{
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

	mate, err := scrapemate.New(
		scrapemate.WithContext(ctx, cancel),
		scrapemate.WithJobProvider(provider),
		scrapemate.WithConcurrency(10),
	)

	if err != nil {
		return err
	}
	return mate.Start()
}
