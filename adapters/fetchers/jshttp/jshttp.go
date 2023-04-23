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

func (o *jsFetch) Session(ctx context.Context) (any, error) {
	bctx, err := newBrowserCtx(o.browser)
	if err != nil {
		return nil, err
	}
	return bctx, nil
}

// Fetch fetches the url specicied by the job and returns the response
func (o *jsFetch) Fetch(ctx context.Context, job scrapemate.IJob) scrapemate.Response {
	browser, ok := GetBrowserFromContext(ctx)
	if !ok {
		var err error
		browser, err = newBrowserCtx(o.browser)
		if err != nil {
			return scrapemate.Response{
				Error: err,
			}
		}
	}
	browser.usage++
	defer func() {
		pages := browser.bwctx.Pages()
		if len(pages) >= 2 {
			for i := 0; i < len(pages)-2; i++ {
				pages[i].Close()
			}
		}
		for _, page := range browser.bwctx.BackgroundPages() {
			page.Close()
		}
	}()
	if job.GetTimeout() > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, job.GetTimeout())
		defer cancel()
	}
	return job.BrowserActions(ctx, browser.page)
}

func GetBrowserFromContext(ctx context.Context) (browserCtx, bool) {
	bctx, ok := ctx.Value("session").(browserCtx)
	return bctx, ok
}

type browserCtx struct {
	bwctx playwright.BrowserContext
	page  playwright.Page
	usage int
}

func newBrowserCtx(browser playwright.Browser) (browserCtx, error) {
	bctx, err := browser.NewContext(playwright.BrowserNewContextOptions{
		Viewport: &playwright.BrowserNewContextOptionsViewport{
			Width:  playwright.Int(1920),
			Height: playwright.Int(1080),
		},
	})
	if err != nil {
		return browserCtx{}, err
	}
	page, err := bctx.NewPage()
	if err != nil {
		return browserCtx{}, err
	}
	ans := browserCtx{
		bwctx: bctx,
		page:  page,
		usage: 0,
	}
	return ans, nil
}
