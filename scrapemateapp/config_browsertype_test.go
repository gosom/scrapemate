package scrapemateapp //nolint:testpackage // Need access to unexported jsOptions.

import "testing"

func TestWithJSBrowserType(t *testing.T) {
	var o jsOptions

	WithJSBrowserType("firefox")(&o)

	if o.BrowserType != "firefox" {
		t.Errorf("BrowserType = %q; want %q", o.BrowserType, "firefox")
	}
}

func TestWithJSExecutablePath(t *testing.T) {
	var o jsOptions

	WithJSExecutablePath("/opt/firefox/firefox")(&o)

	if o.ExecutablePath != "/opt/firefox/firefox" {
		t.Errorf("ExecutablePath = %q; want %q", o.ExecutablePath, "/opt/firefox/firefox")
	}
}

func TestJSOptions_DefaultBrowserTypeEmpty(t *testing.T) {
	// A jsOptions configured without the browser-type option must keep the
	// empty default, which maps to Chromium (backward-compatible).
	var o jsOptions

	WithUA("x")(&o)

	if o.BrowserType != "" {
		t.Errorf("default BrowserType = %q; want empty", o.BrowserType)
	}
}
