package jshttp

import (
	"context"

	"github.com/gosom/scrapemate"
	"github.com/playwright-community/playwright-go"
)

var _ scrapemate.HTTPFetcher = (*jsFetch)(nil)

func New(headless, disableImages, firefox bool) (scrapemate.HTTPFetcher, error) {
	if err := playwright.Install(); err != nil {
		return nil, err
	}

	const poolSize = 10

	ans := jsFetch{
		headless:      headless,
		disableImages: disableImages,
		firefox:       firefox,
		pool:          make(chan *browser, poolSize),
	}

	return &ans, nil
}

type jsFetch struct {
	headless      bool
	disableImages bool
	firefox       bool
	pool          chan *browser
}

func (o *jsFetch) GetBrowser(ctx context.Context) (*browser, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case ans := <-o.pool:
		_ = ans.ctx.ClearCookies()

		for _, p := range ans.ctx.Pages() {
			_ = p.Close()
		}

		for _, bctx := range ans.browser.Contexts() {
			_ = bctx.ClearCookies()
		}

		return ans, nil
	default:
		return newBrowser(o.headless, o.disableImages, o.firefox)
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
	_ = o.ctx.Close()
	_ = o.browser.Close()
	_ = o.pw.Stop()
}

func newBrowser(headless, disableImages, firefox bool) (*browser, error) {
	pw, err := playwright.Run()
	if err != nil {
		return nil, err
	}

	var br playwright.Browser

	if !firefox {
		opts := playwright.BrowserTypeLaunchOptions{
			Headless: playwright.Bool(headless),
			Args: []string{
				`--start-maximized`,
				`--no-sandbox`,
				`--no-default-browser-check`,
				`--enable-automation=false`,
				`--disable-blink-features=AutomationControlled`,
			},
		}

		if disableImages {
			opts.Args = append(opts.Args, `--blink-settings=imagesEnabled=false`)
		}

		br, err = pw.Chromium.Launch(opts)
	} else {
		br, err = pw.Firefox.Launch(playwright.BrowserTypeLaunchOptions{
			Headless: playwright.Bool(headless),
			Args: []string{
				`--start-maximized`,
				`--no-sandbox`,
				`--no-default-browser-check`,
				`--enable-automation=false`,
				`--disable-blink-features=AutomationControlled`,
			},
		})
	}

	if err != nil {
		return nil, err
	}

	const defaultWidth, defaultHeight = 1920, 1080

	const ua = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36"

	bctx, err := br.NewContext(playwright.BrowserNewContextOptions{
		Viewport: &playwright.Size{
			Width:  defaultWidth,
			Height: defaultHeight,
		},
		UserAgent: playwright.String(ua),
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
