package jshttp

import (
	"context"

	"github.com/gosom/scrapemate"
	"github.com/playwright-community/playwright-go"
)

var _ scrapemate.HttpFetcher = (*jsFetch)(nil)

type jsFetch struct {
	pw      *playwright.Playwright
	browser playwright.Browser
}

func New(headless bool) (*jsFetch, error) {
	if err := playwright.Install(); err != nil {
		return nil, err
	}
	pw, err := playwright.Run()
	if err != nil {
		return nil, err
	}
	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(headless),
		Args: []string{
			`--start-maximized`,
			`--no-default-browser-check`,
		},
	})
	if err != nil {
		return nil, err
	}
	return &jsFetch{
		pw:      pw,
		browser: browser,
	}, nil
}

func (o *jsFetch) Fetch(ctx context.Context, job scrapemate.IJob) scrapemate.Response {
	return job.BrowserActions(o.browser)
}
