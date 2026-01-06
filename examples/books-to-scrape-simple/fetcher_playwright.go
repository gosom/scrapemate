//go:build !rod

package main

import (
	"github.com/gosom/scrapemate"
	jsfetcher "github.com/gosom/scrapemate/adapters/fetchers/jshttp"
)

func newJSFetcher(concurrency int, rotator scrapemate.ProxyRotator, _ bool) (scrapemate.HTTPFetcher, error) {
	opts := jsfetcher.JSFetcherOptions{
		Headless:      false,
		DisableImages: false,
		Rotator:       rotator,
		PoolSize:      concurrency,
	}
	return jsfetcher.New(opts)
}
