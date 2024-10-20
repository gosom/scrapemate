package jshttp

import (
	"context"

	"github.com/gosom/scrapemate"
	"github.com/playwright-community/playwright-go"
)

var _ scrapemate.HTTPFetcher = (*jsFetch)(nil)

func New(headless, disableImages bool, rotator scrapemate.ProxyRotator) (scrapemate.HTTPFetcher, error) {
	if err := playwright.Install(); err != nil {
		return nil, err
	}

	const poolSize = 10

	pw, err := playwright.Run()
	if err != nil {
		return nil, err
	}

	ans := jsFetch{
		pw:            pw,
		headless:      headless,
		disableImages: disableImages,
		pool:          make(chan *browser, poolSize),
		rotator:       rotator,
	}

	return &ans, nil
}

type jsFetch struct {
	pw            *playwright.Playwright
	headless      bool
	disableImages bool
	pool          chan *browser
	rotator       scrapemate.ProxyRotator
}

func (o *jsFetch) GetBrowser(ctx context.Context) (*browser, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case ans := <-o.pool:
		return ans, nil
	default:
		ans, err := newBrowser(o.pw, o.headless, o.disableImages, o.rotator)
		if err != nil {
			return nil, err
		}

		return ans, nil
	}
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
	browser playwright.Browser
	ctx     playwright.BrowserContext
}

func (o *browser) Close() {
	_ = o.ctx.Close()
	_ = o.browser.Close()
}

func newBrowser(pw *playwright.Playwright, headless, disableImages bool, rotator scrapemate.ProxyRotator) (*browser, error) {
	opts := playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(headless),
		Args: []string{
			`--start-maximized`,
			`--no-default-browser-check`,
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
		Viewport: &playwright.Size{
			Width:  defaultWidth,
			Height: defaultHeight,
		},
		Proxy: func() *playwright.Proxy {
			if rotator == nil {
				return nil
			}

			next := rotator.Next()

			srv := "socks5://" + next
			username, password := rotator.GetCredentials()

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
