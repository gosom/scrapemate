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
	ans := jsFetch{
		headless: headless,
		pool:     make(chan *browser, 10),
	}
	return &ans, nil
}

type jsFetch struct {
	headless bool
	pool     chan *browser
}

func (o *jsFetch) GetBrowser(ctx context.Context) (*browser, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case ans := <-o.pool:
		return ans, nil
	default:
		return newBrowser(o.headless)
	}
}

func (o *jsFetch) PutBrowser(ctx context.Context, b *browser) {
	select {
	case <-ctx.Done():
		b.Close()
	case o.pool <- b:
	default:
		b.Close()
	}
}

// Fetch fetches the url specicied by the job and returns the response
func (o *jsFetch) Fetch(ctx context.Context, job scrapemate.IJob) scrapemate.Response {
	browser, err := o.GetBrowser(ctx)
	if err != nil {
		return scrapemate.Response{
			Error: err,
		}
	}
	defer o.PutBrowser(ctx, browser)
	if job.GetTimeout() > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, job.GetTimeout())
		defer cancel()
	}
	var page playwright.Page

	if len(browser.ctx.Pages()) > 0 {
		page = browser.ctx.Pages()[0]
		for i := 1; i < len(browser.ctx.Pages()); i++ {
			browser.ctx.Pages()[i].Close()
		}
	} else {
		page, err = browser.ctx.NewPage()
		if err != nil {
			return scrapemate.Response{
				Error: err,
			}
		}
	}
	defer page.Close()
	return job.BrowserActions(ctx, page)
}

type browser struct {
	pw      *playwright.Playwright
	browser playwright.Browser
	ctx     playwright.BrowserContext
}

func (o *browser) Close() {
	o.ctx.Close()
	o.browser.Close()
	o.pw.Stop()
}

func newBrowser(headless bool) (*browser, error) {
	pw, err := playwright.Run()
	if err != nil {
		return nil, err
	}
	br, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(headless),
		Args: []string{
			`--start-maximized`,
			`--no-default-browser-check`,
		},
	})
	if err != nil {
		return nil, err
	}
	bctx, err := br.NewContext(playwright.BrowserNewContextOptions{
		Viewport: &playwright.BrowserNewContextOptionsViewport{
			Width:  playwright.Int(1920),
			Height: playwright.Int(1080),
		},
	})
	if err != nil {
		return nil, err
	}
	ans := browser{
		pw:      pw,
		browser: br,
		ctx:     bctx,
	}
	return &ans, nil
}
