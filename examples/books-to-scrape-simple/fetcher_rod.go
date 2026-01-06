//go:build rod

package main

import (
	"github.com/gosom/scrapemate"
	rodfetcher "github.com/gosom/scrapemate/adapters/fetchers/rodhttp"
)

func newJSFetcher(concurrency int, rotator scrapemate.ProxyRotator, stealth bool) (scrapemate.HTTPFetcher, error) {
	opts := rodfetcher.RodFetcherOptions{
		Headless:      false,
		DisableImages: false,
		Rotator:       rotator,
		PoolSize:      concurrency,
		Stealth:       stealth,
	}
	return rodfetcher.New(opts)
}
