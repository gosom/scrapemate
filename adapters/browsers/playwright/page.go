//go:build !rod

package playwright

import (
	"net/http"
	"time"

	"github.com/playwright-community/playwright-go"

	"github.com/gosom/scrapemate"
)

var _ scrapemate.BrowserPage = (*Page)(nil)
var _ scrapemate.Locator = (*Locator)(nil)

// Page wraps a playwright.Page and implements scrapemate.BrowserPage.
type Page struct {
	page playwright.Page
}

// NewPage creates a new Page wrapper around a playwright.Page.
func NewPage(page playwright.Page) *Page {
	return &Page{page: page}
}

// Goto navigates to a URL and waits for the specified state.
func (p *Page) Goto(url string, waitUntil scrapemate.WaitUntilState) (*scrapemate.PageResponse, error) {
	opts := playwright.PageGotoOptions{
		WaitUntil: toPlaywrightWaitUntil(waitUntil),
	}

	resp, err := p.page.Goto(url, opts)
	if err != nil {
		return nil, err
	}

	body, err := resp.Body()
	if err != nil {
		return nil, err
	}

	headers := make(http.Header, len(resp.Headers()))
	for k, v := range resp.Headers() {
		headers.Add(k, v)
	}

	return &scrapemate.PageResponse{
		URL:        resp.URL(),
		StatusCode: resp.Status(),
		Headers:    headers,
		Body:       body,
	}, nil
}

// URL returns the current page URL.
func (p *Page) URL() string {
	return p.page.URL()
}

// Content returns the full HTML content of the page.
func (p *Page) Content() (string, error) {
	return p.page.Content()
}

// Reload reloads the current page.
func (p *Page) Reload(waitUntil scrapemate.WaitUntilState) error {
	_, err := p.page.Reload(playwright.PageReloadOptions{
		WaitUntil: toPlaywrightWaitUntil(waitUntil),
	})

	return err
}

// Screenshot takes a screenshot of the page.
func (p *Page) Screenshot(fullPage bool) ([]byte, error) {
	return p.page.Screenshot(playwright.PageScreenshotOptions{
		FullPage: playwright.Bool(fullPage),
	})
}

// Eval executes JavaScript in the page context and returns the result.
func (p *Page) Eval(js string, args ...any) (any, error) {
	return p.page.Evaluate(js, args...)
}

// WaitForURL waits until the page URL matches the given pattern.
func (p *Page) WaitForURL(url string, timeout time.Duration) error {
	return p.page.WaitForURL(url, playwright.PageWaitForURLOptions{
		Timeout: playwright.Float(float64(timeout.Milliseconds())),
	})
}

// WaitForSelector waits for an element matching the selector to appear.
func (p *Page) WaitForSelector(selector string, timeout time.Duration) error {
	//nolint:staticcheck // WaitForSelector is deprecated but still needed for compatibility
	_, err := p.page.WaitForSelector(selector, playwright.PageWaitForSelectorOptions{
		Timeout: playwright.Float(float64(timeout.Milliseconds())),
	})

	return err
}

// WaitForTimeout waits for the specified duration.
// Note: This is generally discouraged in favor of waiting for specific conditions.
func (p *Page) WaitForTimeout(timeout time.Duration) {
	//nolint:staticcheck // WaitForTimeout is deprecated but still needed for compatibility
	p.page.WaitForTimeout(float64(timeout.Milliseconds()))
}

// Locator creates a locator for finding elements matching the selector.
func (p *Page) Locator(selector string) scrapemate.Locator {
	return &Locator{locator: p.page.Locator(selector)}
}

// Close closes the page.
func (p *Page) Close() error {
	return p.page.Close()
}

// Unwrap returns the underlying playwright.Page.
func (p *Page) Unwrap() any {
	return p.page
}

// Locator wraps a playwright.Locator and implements scrapemate.Locator.
type Locator struct {
	locator playwright.Locator
}

// Click clicks on the first matching element.
func (l *Locator) Click(timeout time.Duration) error {
	return l.locator.Click(playwright.LocatorClickOptions{
		Timeout: playwright.Float(float64(timeout.Milliseconds())),
	})
}

// Count returns the number of matching elements.
func (l *Locator) Count() (int, error) {
	return l.locator.Count()
}

// First returns a locator for the first matching element.
func (l *Locator) First() scrapemate.Locator {
	return &Locator{locator: l.locator.First()}
}

// toPlaywrightWaitUntil converts scrapemate.WaitUntilState to playwright's wait state.
func toPlaywrightWaitUntil(state scrapemate.WaitUntilState) *playwright.WaitUntilState {
	switch state {
	case scrapemate.WaitUntilLoad:
		return playwright.WaitUntilStateLoad
	case scrapemate.WaitUntilDOMContentLoaded:
		return playwright.WaitUntilStateDomcontentloaded
	case scrapemate.WaitUntilNetworkIdle:
		return playwright.WaitUntilStateNetworkidle
	default:
		return playwright.WaitUntilStateLoad
	}
}
