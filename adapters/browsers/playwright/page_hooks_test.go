package playwright

// Unit tests for the RequestHookProvider implementation on *Page.
//
// Exercising a handler end-to-end (the browser actually firing OnRequest)
// requires a live Playwright browser, so the unit tests here cover the
// capability contract without a browser:
//
//  1. Backward-compat: a BrowserPage that does not implement
//     RequestHookProvider keeps working — the consumer's guarded type assertion
//     returns false and registration is skipped.
//  2. Compile-time guarantee that *Page satisfies RequestHookProvider.

import (
	"testing"
	"time"

	"github.com/gosom/scrapemate"
)

// minimalPage is a BrowserPage that does NOT implement RequestHookProvider. It
// verifies backward compatibility: callers that guard hook registration with a
// type assertion continue to work when the page does not support hooks.
type minimalPage struct{}

func (m *minimalPage) Goto(_ string, _ scrapemate.WaitUntilState) (*scrapemate.PageResponse, error) {
	return &scrapemate.PageResponse{StatusCode: 200}, nil
}
func (m *minimalPage) URL() string                                     { return "" }
func (m *minimalPage) Content() (string, error)                       { return "", nil }
func (m *minimalPage) Reload(_ scrapemate.WaitUntilState) error        { return nil }
func (m *minimalPage) Screenshot(_ bool) ([]byte, error)              { return nil, nil }
func (m *minimalPage) Eval(_ string, _ ...any) (any, error)           { return nil, nil }
func (m *minimalPage) WaitForURL(_ string, _ time.Duration) error      { return nil }
func (m *minimalPage) WaitForSelector(_ string, _ time.Duration) error { return nil }
func (m *minimalPage) WaitForTimeout(_ time.Duration)                  {}
func (m *minimalPage) Locator(_ string) scrapemate.Locator             { return nil }
func (m *minimalPage) Close() error                                    { return nil }
func (m *minimalPage) Unwrap() any                                     { return nil }

// Compile-time assertion: minimalPage satisfies BrowserPage but NOT
// RequestHookProvider.
var _ scrapemate.BrowserPage = (*minimalPage)(nil)

// TestBackwardCompat_NonHookPage verifies that a BrowserPage without
// RequestHookProvider is handled gracefully by the guarded consumer pattern.
func TestBackwardCompat_NonHookPage(t *testing.T) {
	var page scrapemate.BrowserPage = &minimalPage{}

	hookRegistered := false
	if hook, ok := page.(scrapemate.RequestHookProvider); ok {
		hook.OnRequest(func(_ string, _ map[string]string) {})
		hookRegistered = true
	}

	if hookRegistered {
		t.Error("minimalPage must not satisfy RequestHookProvider, so OnRequest must not be registered")
	}
}

// TestPageImplementsRequestHookProvider documents the compile-time guarantee
// (`var _ scrapemate.RequestHookProvider = (*Page)(nil)` in page.go). A live
// *Page cannot be constructed without a Playwright browser process, so the
// runtime behaviour is exercised by integration tests; this records the intent.
func TestPageImplementsRequestHookProvider(t *testing.T) {
	t.Log("*Page implements scrapemate.RequestHookProvider — guaranteed by compile-time assertion in page.go")
}
