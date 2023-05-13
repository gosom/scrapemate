package bookstoscrape

import (
	"context"
	"fmt"

	"github.com/PuerkitoBio/goquery"
	"github.com/gosom/scrapemate"
)

type BookDetailJob struct {
	scrapemate.Job
}

func (o *BookDetailJob) Process(ctx context.Context, resp *scrapemate.Response) (any, []scrapemate.IJob, error) {
	log := scrapemate.GetLoggerFromContext(ctx)
	log.Info("processing book detail job")
	doc, ok := resp.Document.(*goquery.Document)
	if !ok {
		return nil, nil, fmt.Errorf("invalid document type %T expected *goquery.Document", resp.Document)
	}

	product, err := parseProduct(doc)
	if err != nil {
		return nil, nil, err
	}
	product.URL = resp.URL
	product.Screenshot = resp.Screenshot
	return product, nil, nil
}
