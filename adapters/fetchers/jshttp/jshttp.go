package jshttp

import (
	"context"

	"github.com/gosom/scrapemate"
	"github.com/playwright-community/playwright-go"
)

var _ scrapemate.HTTPFetcher = (*jsFetch)(nil)

type JSFetcherOptions struct {
	Headless          bool
	DisableImages     bool
	Rotator           scrapemate.ProxyRotator
	PoolSize          int
	PageReuseLimit    int
	BrowserReuseLimit int
	UserAgent         string
}

func New(params JSFetcherOptions) (scrapemate.HTTPFetcher, error) {
	opts := []*playwright.RunOptions{
		{
			Browsers: []string{"chromium"},
		},
	}

	if err := playwright.Install(opts...); err != nil {
		return nil, err
	}

	pw, err := playwright.Run()
	if err != nil {
		return nil, err
	}

	ans := jsFetch{
		pw:                pw,
		headless:          params.Headless,
		disableImages:     params.DisableImages,
		pool:              make(chan *browser, params.PoolSize),
		rotator:           params.Rotator,
		pageReuseLimit:    params.PageReuseLimit,
		browserReuseLimit: params.BrowserReuseLimit,
		ua:                params.UserAgent,
	}

	for i := 0; i < params.PoolSize; i++ {
		b, err := newBrowser(pw, params.Headless, params.DisableImages, params.Rotator, params.UserAgent)
		if err != nil {
			_ = ans.Close()
			return nil, err
		}

		ans.pool <- b
	}

	return &ans, nil
}

type jsFetch struct {
	pw                *playwright.Playwright
	headless          bool
	disableImages     bool
	pool              chan *browser
	rotator           scrapemate.ProxyRotator
	pageReuseLimit    int
	browserReuseLimit int
	ua                string
}

func (o *jsFetch) GetBrowser(ctx context.Context) (*browser, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case ans := <-o.pool:
		if ans.browser.IsConnected() && (o.browserReuseLimit <= 0 || ans.browserUsage < o.browserReuseLimit) {
			return ans, nil
		}

		ans.browser.Close()
	default:
	}

	return newBrowser(o.pw, o.headless, o.disableImages, o.rotator, o.ua)
}

func (o *jsFetch) Close() error {
	close(o.pool)

	for b := range o.pool {
		b.Close()
	}

	_ = o.pw.Stop()

	return nil
}

func (o *jsFetch) PutBrowser(ctx context.Context, b *browser) {
	if !b.browser.IsConnected() {
		b.Close()

		return
	}

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

	// match the browser default timeout to the job timeout
	if job.GetTimeout() > 0 {
		page.SetDefaultTimeout(float64(job.GetTimeout().Milliseconds()))
	}

	browser.page0Usage++
	browser.browserUsage++

	defer func() {
		if o.pageReuseLimit == 0 || browser.page0Usage >= o.pageReuseLimit {
			_ = page.Close()

			browser.page0Usage = 0
		}
	}()

	return job.BrowserActions(ctx, page)
}

type browser struct {
	browser      playwright.Browser
	ctx          playwright.BrowserContext
	page0Usage   int
	browserUsage int
}

func (o *browser) Close() {
	_ = o.ctx.Close()
	_ = o.browser.Close()
}

func newBrowser(pw *playwright.Playwright, headless, disableImages bool, rotator scrapemate.ProxyRotator, ua string) (*browser, error) {
	opts := playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(headless),
		Args: []string{
			`--start-maximized`,
			`--no-default-browser-check`,
			`--disable-dev-shm-usage`,
			`--no-sandbox`,
			`--disable-setuid-sandbox`,
			`--no-zygote`,
			`--disable-gpu`,
			`--mute-audio`,
			`--disable-extensions`,
			`--single-process`,
			`--disable-breakpad`,
			`--disable-features=TranslateUI,BlinkGenPropertyTrees`,
			`--disable-ipc-flooding-protection`,
			`--enable-features=NetworkService,NetworkServiceInProcess`,
			`--disable-default-apps`,
			`--disable-notifications`,
			`--disable-webgl`,
			`--disable-blink-features=AutomationControlled`,
		},
	}
	if disableImages {
		opts.Args = append(opts.Args, `--blink-settings=imagesEnabled=false`)
	}

	br, err := pw.Chromium.Launch(opts)

	if err != nil {
		return nil, err
	}

	const defaultWidth, defaultHeight = 1920, 1080

	bctx, err := br.NewContext(playwright.BrowserNewContextOptions{
		UserAgent: func() *string {
			if ua == "" {
				defaultUA := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36"

				return &defaultUA
			}

			return &ua
		}(),
		Viewport: &playwright.Size{
			Width:  defaultWidth,
			Height: defaultHeight,
		},
		Proxy: func() *playwright.Proxy {
			if rotator == nil {
				return nil
			}

			next := rotator.Next()

			srv := next.URL
			username := next.Username
			password := next.Password

			return &playwright.Proxy{
				Server: srv,
				Username: func() *string {
					if username == "" {
						return nil
					}
					return &username
				}(),
				Password: func() *string {
					if password == "" {
						return nil
					}
					return &password
				}(),
			}
		}(),
	})
	if err != nil {
		return nil, err
	}

	ans := browser{
		browser: br,
		ctx:     bctx,
	}

	return &ans, nil
}
