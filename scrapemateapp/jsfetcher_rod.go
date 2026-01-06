//go:build rod

package scrapemateapp

import (
	"github.com/gosom/scrapemate"
	rodfetcher "github.com/gosom/scrapemate/adapters/fetchers/rodhttp"
)

func (app *ScrapemateApp) getJSFetcher(rotator scrapemate.ProxyRotator) (scrapemate.HTTPFetcher, error) {
	return rodfetcher.New(rodfetcher.RodFetcherOptions{
		Headless:          !app.cfg.JSOpts.Headfull,
		DisableImages:     app.cfg.JSOpts.DisableImages,
		Rotator:           rotator,
		PoolSize:          app.cfg.Concurrency,
		PageReuseLimit:    app.cfg.PageReuseLimit,
		BrowserReuseLimit: app.cfg.BrowserReuseLimit,
		UserAgent:         app.cfg.JSOpts.UA,
		Stealth:           app.cfg.JSOpts.RodStealth,
	})
}
