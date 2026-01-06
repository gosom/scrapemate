//go:build rod

package rod

import (
	"net/http"
	"regexp"
	"strings"
	"sync"
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
func (p *Page) Goto(url string, waitUntil scrapemate.WaitUntilState) (*scrapemate.PageResponse, error) {
	var (
		statusCode  int
		respHeaders http.Header
		mu          sync.Mutex
	)

	// Enable network domain to capture response
	err := proto.NetworkEnable{}.Call(p.page)
	if err != nil {
		return nil, err
	}

	// Listen for response received events
	go p.page.EachEvent(func(e *proto.NetworkResponseReceived) bool {
		// Only capture the main document response
		if e.Type == proto.NetworkResourceTypeDocument {
			mu.Lock()
			statusCode = e.Response.Status
			respHeaders = make(http.Header)
			for k, v := range e.Response.Headers {
				respHeaders.Add(k, v.String())
			}
			mu.Unlock()
			return true // stop listening
		}
		return false // continue listening
	})()

	// Navigate to the URL
	err = p.page.Navigate(url)
	if err != nil {
		return nil, err
	}

	// Wait for the page to reach the desired state
	err = p.waitUntil(waitUntil)
	if err != nil {
		return nil, err
	}

	// Get the final URL (after redirects)
	info, err := p.page.Info()
	if err != nil {
		return nil, err
	}

	// Get the HTML content
	html, err := p.page.HTML()
	if err != nil {
		return nil, err
	}

	mu.Lock()
	// Default to 200 if we didn't capture the status (e.g., from cache)
	if statusCode == 0 {
		statusCode = 200
	}

	if respHeaders == nil {
		respHeaders = make(http.Header)
	}
	mu.Unlock()

	return &scrapemate.PageResponse{
		URL:        info.URL,
		StatusCode: statusCode,
		Headers:    respHeaders,
		Body:       []byte(html),
	}, nil
}

// waitUntil waits for the page to reach the specified state.
func (p *Page) waitUntil(state scrapemate.WaitUntilState) error {
	switch state {
	case scrapemate.WaitUntilLoad:
		return p.page.WaitLoad()
	case scrapemate.WaitUntilDOMContentLoaded:
		// Wait for DOMContentLoaded event which is faster than full load
		p.page.WaitNavigation(proto.PageLifecycleEventNameDOMContentLoaded)()
		return nil
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
// It handles both function expressions (arrow functions) and direct expressions/IIFEs.
func (p *Page) Eval(js string, args ...any) (any, error) {
	trimmed := strings.TrimSpace(js)

	// Check if it's an arrow function or regular function that rod.Eval expects
	// rod.Eval wraps the JS in: function() { return (JS).apply(this, arguments) }
	// This works for: () => expr, (a,b) => expr, function() {}, async () => {}
	// But NOT for IIFEs like (function(){})() or direct expressions like "1+1"
	isFunction := isJSFunctionExpression(trimmed)

	if isFunction && len(args) == 0 {
		// Use rod's Eval which handles arrow functions properly
		result, err := p.page.Eval(js)
		if err != nil {
			return nil, err
		}

		return result.Value.Val(), nil
	}

	if isFunction && len(args) > 0 {
		// Use rod's Eval with arguments
		rodArgs := make([]interface{}, len(args))
		for i, arg := range args {
			rodArgs[i] = arg
		}

		result, err := p.page.Eval(js, rodArgs...)
		if err != nil {
			return nil, err
		}

		return result.Value.Val(), nil
	}

	// For IIFEs and direct expressions, use RuntimeEvaluate
	result, err := proto.RuntimeEvaluate{
		Expression:    js,
		ReturnByValue: true,
		AwaitPromise:  true,
	}.Call(p.page)
	if err != nil {
		return nil, err
	}

	if result.ExceptionDetails != nil {
		return nil, &EvalError{Message: result.ExceptionDetails.Text}
	}

	return result.Result.Value.Val(), nil
}

// isJSFunctionExpression checks if the JS code is a function expression
// that rod.Eval can handle (arrow functions, function declarations).
// Returns false for IIFEs and direct expressions.
func isJSFunctionExpression(js string) bool {
	// Arrow functions: () =>, (a) =>, (a, b) =>, async () =>
	// But NOT IIFEs: (() => {})(), (function(){})()
	if strings.HasPrefix(js, "(") {
		// Could be IIFE: (function(){})() or (() => {})()
		// Or could be arrow function with parens: (a, b) => {}
		// Check if it ends with () which indicates IIFE invocation
		if strings.HasSuffix(js, "()") || strings.HasSuffix(js, "();") {
			return false
		}
		// Check for arrow after closing paren - this is a function expression
		// e.g., "(a, b) => a + b"
		parenDepth := 0
		for i, ch := range js {
			if ch == '(' {
				parenDepth++
			} else if ch == ')' {
				parenDepth--
				if parenDepth == 0 {
					// Check what comes after the closing paren
					rest := strings.TrimSpace(js[i+1:])
					if strings.HasPrefix(rest, "=>") {
						return true
					}
					// Otherwise it's likely an IIFE or expression
					return false
				}
			}
		}
	}

	// Simple arrow functions: () =>, async () =>
	if strings.HasPrefix(js, "()") || strings.HasPrefix(js, "async ()") ||
		strings.HasPrefix(js, "async()") {
		return true
	}

	// Function keyword
	if strings.HasPrefix(js, "function") || strings.HasPrefix(js, "async function") {
		// But not IIFE: (function(){})()
		return !strings.HasSuffix(strings.TrimSpace(js), "()")
	}

	return false
}

// EvalError represents a JavaScript evaluation error.
type EvalError struct {
	Message string
}

func (e *EvalError) Error() string {
	return "eval error: " + e.Message
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
func (p *Page) WaitForURL(pattern string, timeout time.Duration) error {
	// Compile the pattern as a regex (supports glob-like patterns)
	// Convert glob pattern to regex if needed
	regexPattern := globToRegex(pattern)

	re, err := regexp.Compile(regexPattern)
	if err != nil {
		// If pattern is not valid regex, treat as literal string match
		re = regexp.MustCompile(regexp.QuoteMeta(pattern))
	}

	deadline := time.Now().Add(timeout)
	pollInterval := 100 * time.Millisecond

	for time.Now().Before(deadline) {
		currentURL := p.URL()
		if re.MatchString(currentURL) {
			return nil
		}

		remaining := time.Until(deadline)
		if remaining < pollInterval {
			pollInterval = remaining
		}

		time.Sleep(pollInterval)
	}

	return &TimeoutError{Message: "timeout waiting for URL to match pattern: " + pattern}
}

// globToRegex converts a glob pattern to a regex pattern.
func globToRegex(glob string) string {
	// If it already looks like a regex (contains ^ or $), return as-is
	if strings.HasPrefix(glob, "^") || strings.HasSuffix(glob, "$") {
		return glob
	}

	// Convert glob wildcards to regex
	var result strings.Builder
	result.WriteString("^")

	for i := 0; i < len(glob); i++ {
		c := glob[i]
		switch c {
		case '*':
			if i+1 < len(glob) && glob[i+1] == '*' {
				// ** matches anything including /
				result.WriteString(".*")
				i++
			} else {
				// * matches anything except /
				result.WriteString("[^/]*")
			}
		case '?':
			result.WriteString("[^/]")
		case '.', '+', '^', '$', '(', ')', '[', ']', '{', '}', '|', '\\':
			result.WriteByte('\\')
			result.WriteByte(c)
		default:
			result.WriteByte(c)
		}
	}

	result.WriteString("$")

	return result.String()
}

// TimeoutError represents a timeout error.
type TimeoutError struct {
	Message string
}

func (e *TimeoutError) Error() string {
	return e.Message
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
