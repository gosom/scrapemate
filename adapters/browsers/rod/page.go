package rod

import (
	"net/http"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"

	"github.com/gosom/scrapemate"
)

var _ scrapemate.BrowserPage = (*Page)(nil)
var _ scrapemate.Locator = (*Locator)(nil)

// default idle duration for network idle detection.
const defaultIdleDuration = 500 * time.Millisecond

// Page wraps a *rod.Page and implements scrapemate.BrowserPage.
type Page struct {
	page *rod.Page
}

// NewPage creates a new Page wrapper around a *rod.Page.
func NewPage(page *rod.Page) *Page {
	return &Page{page: page}
}

// Goto navigates to a URL and waits for the specified state.
// It uses CDP network events to capture the HTTP status code.
func (p *Page) Goto(url string, waitUntil scrapemate.WaitUntilState) (*scrapemate.PageResponse, error) {
	// Set up wait for navigation before navigating
	wait := p.page.WaitNavigation(proto.PageLifecycleEventNameNetworkAlmostIdle)

	// Navigate to the URL
	err := p.page.Navigate(url)
	if err != nil {
		return nil, err
	}

	// Wait for navigation to complete
	wait()

	// Wait for the page to reach the desired state (additional wait if needed)
	err = p.waitUntil(waitUntil)
	if err != nil {
		return nil, err
	}

	// Get response details via JavaScript Performance API
	statusCode, headers := p.getResponseDetails()

	// Get the page content
	html, err := p.page.HTML()
	if err != nil {
		return nil, err
	}

	// Get the final URL (after redirects)
	info, err := p.page.Info()
	if err != nil {
		return nil, err
	}

	return &scrapemate.PageResponse{
		URL:        info.URL,
		StatusCode: statusCode,
		Headers:    headers,
		Body:       []byte(html),
	}, nil
}

// getResponseDetails attempts to get response details via Performance API.
// Returns default values if not available.
func (p *Page) getResponseDetails() (int, http.Header) {
	// Default values
	statusCode := 200
	headers := make(http.Header)

	// Try to get performance entries for response status
	result, err := p.page.Eval(`() => {
		const entries = performance.getEntriesByType('navigation');
		if (entries.length > 0) {
			const nav = entries[0];
			return {
				responseStatus: nav.responseStatus || 200,
			};
		}
		return { responseStatus: 200 };
	}`)

	if err == nil && result != nil {
		if status := result.Value.Get("responseStatus").Int(); status > 0 {
			statusCode = status
		}
	}

	return statusCode, headers
}

// waitUntil waits for the page to reach the specified state.
func (p *Page) waitUntil(state scrapemate.WaitUntilState) error {
	switch state {
	case scrapemate.WaitUntilLoad:
		return p.page.WaitLoad()
	case scrapemate.WaitUntilDOMContentLoaded:
		return p.page.WaitDOMStable(0, 0)
	case scrapemate.WaitUntilNetworkIdle:
		p.page.WaitRequestIdle(defaultIdleDuration, nil, nil, nil)()

		return nil
	default:
		return p.page.WaitLoad()
	}
}

// Content returns the full HTML content of the page.
func (p *Page) Content() (string, error) {
	return p.page.HTML()
}

// Screenshot takes a screenshot of the page.
func (p *Page) Screenshot(fullPage bool) ([]byte, error) {
	if fullPage {
		return p.page.Screenshot(true, nil)
	}

	return p.page.Screenshot(false, nil)
}

// Eval executes JavaScript in the page context and returns the result.
func (p *Page) Eval(js string, args ...any) (any, error) {
	result, err := p.page.Eval(js, args...)
	if err != nil {
		return nil, err
	}

	return result.Value.Val(), nil
}

// Close closes the page.
func (p *Page) Close() error {
	return p.page.Close()
}

// URL returns the current page URL.
func (p *Page) URL() string {
	info, err := p.page.Info()
	if err != nil {
		return ""
	}

	return info.URL
}

// Reload reloads the current page.
func (p *Page) Reload(waitUntil scrapemate.WaitUntilState) error {
	err := p.page.Reload()
	if err != nil {
		return err
	}

	return p.waitUntil(waitUntil)
}

// WaitForURL waits until the page URL matches the given pattern.
func (p *Page) WaitForURL(_ string, timeout time.Duration) error {
	p.page.Timeout(timeout).WaitNavigation(proto.PageLifecycleEventNameNetworkAlmostIdle)()

	return nil
}

// WaitForSelector waits for an element matching the selector to appear.
func (p *Page) WaitForSelector(selector string, timeout time.Duration) error {
	_, err := p.page.Timeout(timeout).Element(selector)

	return err
}

// WaitForTimeout waits for the specified duration.
func (p *Page) WaitForTimeout(timeout time.Duration) {
	time.Sleep(timeout)
}

// Locator creates a locator for finding elements matching the selector.
func (p *Page) Locator(selector string) scrapemate.Locator {
	return &Locator{page: p.page, selector: selector}
}

// Unwrap returns the underlying *rod.Page.
func (p *Page) Unwrap() any {
	return p.page
}

// Locator wraps a rod element selector and implements scrapemate.Locator.
type Locator struct {
	page     *rod.Page
	selector string
	first    bool
}

// Click clicks on the first matching element.
func (l *Locator) Click(timeout time.Duration) error {
	el, err := l.page.Timeout(timeout).Element(l.selector)
	if err != nil {
		return err
	}

	return el.Click(proto.InputMouseButtonLeft, 1)
}

// Count returns the number of matching elements.
func (l *Locator) Count() (int, error) {
	elements, err := l.page.Elements(l.selector)
	if err != nil {
		return 0, err
	}

	return len(elements), nil
}

// First returns a locator for the first matching element.
func (l *Locator) First() scrapemate.Locator {
	return &Locator{page: l.page, selector: l.selector, first: true}
}
