package scrapemate

import (
	"context"
)

// JobProvider is an interface for job providers
// a job provider is a service that provides jobs to scrapemate
// scrapemate will call the job provider to get jobs
//
//go:generate mockgen -destination=mock/mock_provider.go -package=mock . JobProvider
type JobProvider interface {
	Jobs(ctx context.Context) (<-chan IJob, <-chan error)
	// Push pushes a job to the job provider
	Push(ctx context.Context, job IJob) error
}

// HttpFetcher is an interface for http fetchers
//
//go:generate mockgen -destination=mock/mock_http_fetcher.go -package=mock . HttpFetcher
type HttpFetcher interface {
	Fetch(ctx context.Context, job IJob) Response
}

// HtmlParser is an interface for html parsers
//
//go:generate mockgen -destination=mock/mock_parser.go -package=mock . HtmlParser
type HtmlParser interface {
	Parse(ctx context.Context, body []byte) (any, error)
}

// Cacher is an interface for cache
//
//go:generate mockgen -destination=mock/mock_cacher.go -package=mock . Cacher
type Cacher interface {
	Get(ctx context.Context, key string) (Response, error)
	Set(ctx context.Context, key string, value Response) error
}
