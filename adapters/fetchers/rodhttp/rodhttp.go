//go:build rod

package rodhttp

import (
	"context"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"github.com/go-rod/stealth"

	"github.com/gosom/scrapemate"
	rodadapter "github.com/gosom/scrapemate/adapters/browsers/rod"
)

var _ scrapemate.HTTPFetcher = (*rodFetch)(nil)

const (
	defaultWidth     = 1920
	defaultHeight    = 1080
	defaultUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
)

// RodFetcherOptions contains options for creating a new rod fetcher.
type RodFetcherOptions struct {
	// Headless runs the browser in headless mode.
	Headless bool
	// DisableImages disables image loading.
	DisableImages bool
	// Rotator is the proxy rotator.
	Rotator scrapemate.ProxyRotator
	// PoolSize is the number of browsers in the pool.
	PoolSize int
	// PageReuseLimit is the number of times a page can be reused.
	PageReuseLimit int
	// BrowserReuseLimit is the number of times a browser can be reused.
	BrowserReuseLimit int
	// UserAgent is the user agent to use.
	UserAgent string
	// Stealth enables stealth mode to avoid bot detection.
	Stealth bool
}

// New creates a new rod-based HTTP fetcher.
func New(params RodFetcherOptions) (scrapemate.HTTPFetcher, error) {
	if params.PoolSize <= 0 {
		params.PoolSize = 1
	}

	if params.UserAgent == "" {
		params.UserAgent = defaultUserAgent
	}

	ans := &rodFetch{
		headless:          params.Headless,
		disableImages:     params.DisableImages,
		pool:              make(chan *browser, params.PoolSize),
		rotator:           params.Rotator,
		pageReuseLimit:    params.PageReuseLimit,
		browserReuseLimit: params.BrowserReuseLimit,
		ua:                params.UserAgent,
		stealth:           params.Stealth,
	}

	// Pre-populate the browser pool
	for range params.PoolSize {
		b, err := ans.newBrowser()
		if err != nil {
			_ = ans.Close()

			return nil, err
		}

		ans.pool <- b
	}

	return ans, nil
}

type rodFetch struct {
	headless          bool
	disableImages     bool
	pool              chan *browser
	rotator           scrapemate.ProxyRotator
	pageReuseLimit    int
	browserReuseLimit int
	ua                string
	stealth           bool
}

func (o *rodFetch) getBrowser(ctx context.Context) (*browser, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case b := <-o.pool:
		// Check if browser is still usable
		if o.browserReuseLimit <= 0 || b.browserUsage < o.browserReuseLimit {
			return b, nil
		}

		b.Close()
	default:
	}

	return o.newBrowser()
}

func (o *rodFetch) putBrowser(ctx context.Context, b *browser) {
	select {
	case <-ctx.Done():
		b.Close()
	case o.pool <- b:
	default:
		b.Close()
	}
}

// Close closes all browsers in the pool.
func (o *rodFetch) Close() error {
	close(o.pool)

	for b := range o.pool {
		b.Close()
	}

	return nil
}

// Fetch fetches the URL specified by the job and returns the response.
func (o *rodFetch) Fetch(ctx context.Context, job scrapemate.IJob) scrapemate.Response {
	b, err := o.getBrowser(ctx)
	if err != nil {
		return scrapemate.Response{Error: err}
	}

	defer o.putBrowser(ctx, b)

	if job.GetTimeout() > 0 {
		var cancel context.CancelFunc

		ctx, cancel = context.WithTimeout(ctx, job.GetTimeout())
		defer cancel()
	}

	// Get or create a page
	page, err := o.getPage(b)
	if err != nil {
		return scrapemate.Response{Error: err}
	}

	// Set timeout on the page
	if job.GetTimeout() > 0 {
		page = page.Timeout(job.GetTimeout())
	}

	b.pageUsage++
	b.browserUsage++

	defer func() {
		if o.pageReuseLimit > 0 && b.pageUsage >= o.pageReuseLimit {
			_ = page.Close()

			b.page = nil
			b.pageUsage = 0
		}
	}()

	wrappedPage := rodadapter.NewPage(page)

	return job.BrowserActions(ctx, wrappedPage)
}

func (o *rodFetch) getPage(b *browser) (*rod.Page, error) {
	if b.page != nil {
		return b.page, nil
	}

	var (
		page *rod.Page
		err  error
	)

	if o.stealth {
		page, err = stealth.Page(b.browser)
	} else {
		page, err = b.browser.Page(proto.TargetCreateTarget{URL: "about:blank"})
	}

	if err != nil {
		return nil, err
	}

	b.page = page

	return page, nil
}

func (o *rodFetch) newBrowser() (*browser, error) {
	l := launcher.New().
		Headless(o.headless).
		Set("no-sandbox").
		Set("disable-setuid-sandbox").
		Set("disable-dev-shm-usage").
		Set("disable-gpu").
		Set("no-first-run").
		Set("disable-extensions").
		Set("mute-audio").
		Set("disable-background-networking").
		Set("disable-sync").
		Set("disable-blink-features", "AutomationControlled").
		Set("ignore-certificate-errors").
		// Flags for containerized/restricted environments
		Set("no-zygote").
		Set("single-process").
		// Flags for reliable scraping timing
		Set("disable-background-timer-throttling").
		Set("disable-backgrounding-occluded-windows").
		Set("disable-renderer-backgrounding").
		// Allow popups for scraping flows
		Set("disable-popup-blocking")

	if o.disableImages {
		l = l.Set("blink-settings", "imagesEnabled=false")
	}

	// Handle proxy
	if o.rotator != nil {
		proxies := o.rotator.Proxies()
		if len(proxies) > 0 {
			// For simplicity, use the first proxy
			// TODO: Implement proxy rotation per browser
			proxy := o.rotator.Next()

			l = l.Proxy(proxy.FullURL())
		}
	}

	controlURL, err := l.Launch()
	if err != nil {
		return nil, err
	}

	br := rod.New().ControlURL(controlURL)

	err = br.Connect()
	if err != nil {
		return nil, err
	}

	// Create initial page (with stealth if enabled)
	var page *rod.Page

	if o.stealth {
		page, err = stealth.Page(br)
	} else {
		page, err = br.Page(proto.TargetCreateTarget{URL: "about:blank"})
	}

	if err != nil {
		br.Close()

		return nil, err
	}

	// Set viewport
	err = page.SetViewport(&proto.EmulationSetDeviceMetricsOverride{
		Width:  defaultWidth,
		Height: defaultHeight,
	})
	if err != nil {
		br.Close()

		return nil, err
	}

	// Set user agent
	err = page.SetUserAgent(&proto.NetworkSetUserAgentOverride{
		UserAgent: o.ua,
	})
	if err != nil {
		br.Close()

		return nil, err
	}

	return &browser{
		browser:  br,
		launcher: l,
		page:     page,
	}, nil
}

type browser struct {
	browser      *rod.Browser
	launcher     *launcher.Launcher
	page         *rod.Page
	pageUsage    int
	browserUsage int
}

func (b *browser) Close() {
	if b.page != nil {
		_ = b.page.Close()
	}

	if b.browser != nil {
		_ = b.browser.Close()
	}

	if b.launcher != nil {
		b.launcher.Cleanup()
	}
}
