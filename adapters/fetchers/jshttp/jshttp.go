package jshttp

import (
	"context"
	"errors"

	"github.com/playwright-community/playwright-go"

	"github.com/gosom/scrapemate"
	playwrightadapter "github.com/gosom/scrapemate/adapters/browsers/playwright"
)

var _ scrapemate.HTTPFetcher = (*jsFetch)(nil)

type JSFetcherOptions struct {
	Headless           bool
	DisableImages      bool
	Rotator            scrapemate.ProxyRotator
	PoolSize           int
	MaxPagesPerBrowser int
	PageReuseLimit     int
	BrowserReuseLimit  int
	UserAgent          string
	// BrowserType selects the Playwright browser engine. Accepted values are
	// "chromium" (the default, also for the empty string), "firefox" and
	// "webkit". Callers that never set this field keep the existing Chromium
	// behaviour unchanged.
	BrowserType string
	// ExecutablePath, when non-empty, overrides the Playwright-managed browser
	// binary (for example a custom Firefox build). Empty uses the bundled binary.
	ExecutablePath string
}

//nolint:gocritic // Keep value parameter to preserve the public constructor API.
func New(params JSFetcherOptions) (scrapemate.HTTPFetcher, error) {
	opts := []*playwright.RunOptions{
		{
			Browsers: browsersToInstall(params.BrowserType),
			Verbose:  true,
		},
	}

	if err := playwright.Install(opts...); err != nil {
		return nil, err
	}

	pw, err := playwright.Run()
	if err != nil {
		return nil, err
	}

	var pool *ProxyPool

	if params.Rotator != nil {
		proxies := params.Rotator.Proxies()

		if len(proxies) > 0 {
			pool, err = NewProxyPool(proxies)
			if err != nil {
				return nil, err
			}
		}
	}

	maxPagesPerBrowser := params.MaxPagesPerBrowser
	if maxPagesPerBrowser < 1 {
		maxPagesPerBrowser = 1
	}

	ans := jsFetch{
		pw:                 pw,
		headless:           params.Headless,
		disableImages:      params.DisableImages,
		pageReuseLimit:     params.PageReuseLimit,
		browserReuseLimit:  params.BrowserReuseLimit,
		ua:                 params.UserAgent,
		proxyPool:          pool,
		rotator:            params.Rotator,
		maxPagesPerBrowser: maxPagesPerBrowser,
	}

	if maxPagesPerBrowser > 1 {
		ans.pageSlots, err = newPageSlotPool(pageSlotPoolConfig{
			poolSize:           params.PoolSize,
			maxPagesPerBrowser: maxPagesPerBrowser,
			factory: playwrightSlotFactory{
				pw:             pw,
				headless:       params.Headless,
				disableImages:  params.DisableImages,
				proxyPool:      pool,
				ua:             params.UserAgent,
				browserType:    params.BrowserType,
				executablePath: params.ExecutablePath,
			},
		})
		if err != nil {
			_ = ans.Close()

			return nil, err
		}

		return &ans, nil
	}

	ans.slots = make(chan *sessionSlot, params.PoolSize)

	sessionFactory := &playwrightRuntimeFactory{
		pw:             pw,
		headless:       params.Headless,
		disableImages:  params.DisableImages,
		proxyPool:      pool,
		ua:             params.UserAgent,
		browserType:    params.BrowserType,
		executablePath: params.ExecutablePath,
	}
	ans.factory = sessionFactory

	for range params.PoolSize {
		ans.slots <- newSessionSlot(sessionFactory)
	}

	return &ans, nil
}

type jsFetch struct {
	pw                *playwright.Playwright
	headless          bool
	disableImages     bool
	pageReuseLimit    int
	browserReuseLimit int
	ua                string
	proxyPool         *ProxyPool
	factory           runtimeFactory
	slots             chan *sessionSlot
	pageSlots         *pageSlotPool

	rotator            scrapemate.ProxyRotator
	maxPagesPerBrowser int
}

func (o *jsFetch) getSlot(ctx context.Context) (*sessionSlot, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case slot := <-o.slots:
		return slot, nil
	}
}

func (o *jsFetch) putSlot(ctx context.Context, slot *sessionSlot) {
	select {
	case <-ctx.Done():
		_ = slot.close()
	case o.slots <- slot:
	}
}

func (o *jsFetch) Close() error {
	if o.pageSlots != nil {
		o.pageSlots.close()
	}

	if o.slots != nil {
		close(o.slots)

		for slot := range o.slots {
			slot.close()
		}
	}

	_ = o.pw.Stop()

	return nil
}

// Fetch fetches the url specicied by the job and returns the response
func (o *jsFetch) Fetch(ctx context.Context, job scrapemate.IJob) scrapemate.Response {
	if o.maxPagesPerBrowser > 1 {
		return o.fetchWithPageSlot(ctx, job)
	}

	slot, err := o.getSlot(ctx)
	if err != nil {
		return scrapemate.Response{
			Error: err,
		}
	}

	defer o.putSlot(ctx, slot)

	p, err := slot.acquirePage(ctx)
	if err != nil {
		return scrapemate.Response{
			Error: err,
		}
	}

	pp, ok := p.(*playwrightPage)
	if !ok {
		return scrapemate.Response{
			Error: errors.New("unexpected page type"),
		}
	}

	if job.GetTimeout() > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, job.GetTimeout())

		defer cancel()

		pp.playwrightPage().SetDefaultTimeout(float64(job.GetTimeout().Milliseconds()))
	}

	wrappedPage := playwrightadapter.NewPage(pp.playwrightPage())

	resp := job.BrowserActions(ctx, wrappedPage)

	if cleanErr := slot.release(ctx); cleanErr != nil && resp.Error == nil {
		resp.Error = cleanErr
	}

	return resp
}

func (o *jsFetch) fetchWithPageSlot(ctx context.Context, job scrapemate.IJob) scrapemate.Response {
	if job.GetTimeout() > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, job.GetTimeout())

		defer cancel()
	}

	lease, err := o.pageSlots.acquire(ctx)
	if err != nil {
		return scrapemate.Response{Error: err}
	}

	defer lease.release(ctx)

	page, err := lease.slot.ctx.NewPage()
	if err != nil {
		return scrapemate.Response{Error: err}
	}

	defer page.Close()

	if job.GetTimeout() > 0 {
		page.SetDefaultTimeout(float64(job.GetTimeout().Milliseconds()))
	}

	lease.slot.mu.Lock()
	lease.slot.browserUsage++
	lease.slot.mu.Unlock()

	return job.BrowserActions(ctx, playwrightadapter.NewPage(page))
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

// browsersToInstall maps a BrowserType value to the Playwright install list.
// An empty string or "chromium" both install Chromium so that callers that
// never set BrowserType are unaffected.
func browsersToInstall(browserType string) []string {
	switch browserType {
	case "firefox":
		return []string{"firefox"}
	case "webkit":
		return []string{"webkit"}
	default:
		return []string{"chromium"}
	}
}

// chromiumLaunchArgs are the Chromium-specific command-line flags. They are only
// passed when launching Chromium: Firefox and WebKit reject or mishandle these
// flags, and forwarding them causes Firefox to hang on the first NewPage call.
func chromiumLaunchArgs(disableImages bool) []string {
	args := []string{
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
		"--enable-features=NetworkService",
		`--disable-default-apps`,
		`--disable-notifications`,
		`--disable-webgl`,
		`--disable-blink-features=AutomationControlled`,
		"--ignore-certificate-errors",
		"--ignore-certificate-errors-spki-list",
		"--disable-web-security",
	}
	if disableImages {
		args = append(args, `--blink-settings=imagesEnabled=false`)
	}

	return args
}

// browserTypeFor returns the playwright.BrowserType for the configured engine.
// Empty or "chromium" return Chromium so existing callers are unaffected.
func browserTypeFor(pw *playwright.Playwright, browserType string) playwright.BrowserType {
	switch browserType {
	case "firefox":
		return pw.Firefox
	case "webkit":
		return pw.WebKit
	default:
		return pw.Chromium
	}
}

func newBrowser(pw *playwright.Playwright, headless, disableImages bool, proxyPool *ProxyPool, ua, browserType, executablePath string) (*browser, error) {
	opts := playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(headless),
	}

	// Chromium launch flags only apply to Chromium. Firefox/WebKit use the
	// Playwright engine defaults; forwarding Chromium flags hangs Firefox at
	// the first NewPage.
	if browserType == "" || browserType == "chromium" {
		opts.Args = chromiumLaunchArgs(disableImages)
	}

	if executablePath != "" {
		opts.ExecutablePath = playwright.String(executablePath)
	}

	br, err := browserTypeFor(pw, browserType).Launch(opts)
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
			if proxyPool != nil {
				authProxy := proxyPool.Next()

				addr := authProxy.Address()

				return &playwright.Proxy{
					Server: addr,
				}
			}

			return nil
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
