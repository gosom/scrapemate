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

// HTTPFetcher is an interface for http fetchers
//
//go:generate mockgen -destination=mock/mock_http_fetcher.go -package=mock . HTTPFetcher
type HTTPFetcher interface {
	Fetch(ctx context.Context, job IJob) Response
}

// HTMLParser is an interface for html parsers
//
//go:generate mockgen -destination=mock/mock_parser.go -package=mock . HTMLParser
type HTMLParser interface {
	Parse(ctx context.Context, body []byte) (any, error)
}

// Cacher is an interface for cache
//
//go:generate mockgen -destination=mock/mock_cacher.go -package=mock . Cacher
type Cacher interface {
	Close() error
	Get(ctx context.Context, key string) (Response, error)
	Set(ctx context.Context, key string, value *Response) error
}

// ResultWriter is an interface for result writers
//
//go:generate mockgen -destination=mock/mock_writer.go -package=mock . ResultWriter
type ResultWriter interface {
	Run(ctx context.Context, in <-chan Result) error
}
