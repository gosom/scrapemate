package jshttp

import (
	"context"

	"github.com/gosom/scrapemate"
	"github.com/playwright-community/playwright-go"
)

var _ scrapemate.HttpFetcher = (*jsFetch)(nil)

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
	ans := jsFetch{
		pw:      pw,
		browser: browser,
	}
	return &ans, nil
}

type jsFetch struct {
	pw      *playwright.Playwright
	browser playwright.Browser
}

// Fetch fetches the url specicied by the job and returns the response
func (o *jsFetch) Fetch(ctx context.Context, job scrapemate.IJob) scrapemate.Response {
	browserCtx, err := o.browser.NewContext()
	if err != nil {
		return scrapemate.Response{Error: err}
	}
	if job.GetTimeout() > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, job.GetTimeout())
		defer cancel()
	}
	page, err := o.newPage(browserCtx)
	if err != nil {
		return scrapemate.Response{Error: err}
	}
	defer page.Close()

	return job.BrowserActions(ctx, page)
}

func (o *jsFetch) newPage(bctx playwright.BrowserContext) (playwright.Page, error) {
	page, err := bctx.NewPage()
	if err != nil {
		return nil, err
	}
	if err := page.SetViewportSize(1920, 1080); err != nil {
		return nil, err
	}
	return page, nil
}
