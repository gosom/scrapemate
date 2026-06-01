# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- `jshttp`: optional Firefox/WebKit browser engine for JS rendering via
  `JSFetcherOptions.BrowserType` (`"chromium"` default, `"firefox"`, `"webkit"`)
  and `JSFetcherOptions.ExecutablePath` to override the browser binary. Exposed
  through `scrapemateapp.WithJSBrowserType(...)` and
  `scrapemateapp.WithJSExecutablePath(...)` as `WithJS` sub-options. Chromium
  launch flags are only applied to Chromium; Firefox/WebKit use the Playwright
  engine defaults. Backward-compatible — the empty default keeps Chromium.

### Removed

- Rod browser support, build tags, and related fetcher/page implementations
- Rod-specific example wiring and documentation

### Changed

- JavaScript rendering is now Playwright-only
- `scrapemateapp.WithBrowserEngine()` and `scrapemateapp.WithRodStealth()` remain as deprecated no-op compatibility shims

### Fixed

- `jshttp`: `page.Close()` is now time-bounded (5 s default via `closeWithTimeout`).
  A wedged Playwright driver (e.g. EPIPE after a browser crash) could make
  `page.Close()` block indefinitely, stalling the worker goroutine and preventing
  the fetcher from returning. The cleanup goroutine is now abandoned after the
  deadline so the worker is freed. Backward-compatible — no API change; in the
  healthy case `Close` returns within milliseconds and the deadline is never reached.

## [1.0.0] - 2026-01-06

### Breaking Changes

#### Browser Interface Abstraction

The `IJob` interface signature has changed to support multiple browser engines:

**Before (v0.9.6):**
```go
BrowserActions(ctx context.Context, page playwright.Page) Response
```

**After (v1.0.0):**
```go
BrowserActions(ctx context.Context, page BrowserPage) Response
```

#### Migration Guide

1. **Update the method signature** in your job types:

```go
// Before
func (j *MyJob) BrowserActions(ctx context.Context, page playwright.Page) scrapemate.Response {
    // ...
}

// After
func (j *MyJob) BrowserActions(ctx context.Context, page scrapemate.BrowserPage) scrapemate.Response {
    // ...
}
```

2. **Update navigation calls** to use the new interface:

```go
// Before
page.Goto("https://example.com", playwright.PageGotoOptions{
    WaitUntil: playwright.WaitUntilStateNetworkidle,
})

// After
resp, err := page.Goto("https://example.com", scrapemate.WaitUntilNetworkIdle)
```

3. **If you need the underlying browser page**, use `Unwrap()`:

```go
// For Playwright-specific features
pwPage := page.Unwrap().(playwright.Page)

// For Rod-specific features (when compiled with -tags rod)
rodPage := page.Unwrap().(*rod.Page)
```

#### Browser Engine Selection

Browser engine selection has changed from runtime configuration to compile-time build tags:

**Before (v0.9.6):**
```go
// Runtime selection (no longer supported)
scrapemateapp.WithBrowserEngine(scrapemateapp.BrowserEngineRod)
```

**After (v1.0.0):**
```bash
# Playwright (default)
go build ./...

# Rod
go build -tags rod ./...
```

### Added

- New `BrowserPage` interface providing a unified API for browser automation
- New `Locator` interface for element selection
- Support for Rod browser engine via build tags
- `WaitUntilState` constants: `WaitUntilLoad`, `WaitUntilDOMContentLoaded`, `WaitUntilNetworkIdle`
- `PageResponse` struct with `URL`, `StatusCode`, `Headers`, and `Body` fields
- Rod stealth mode support via `-stealth` flag
- Chrome flags for containerized environments (Rod)

### Changed

- `BrowserActions` now receives `scrapemate.BrowserPage` instead of `playwright.Page`
- Browser engine is now selected at compile time using build tags
- Rod implementation now returns actual response data (status code, headers, body)

### Removed

- Runtime browser engine selection via `WithBrowserEngine()` (function exists but is a no-op)

## [0.9.6] and earlier

See [GitHub Releases](https://github.com/gosom/scrapemate/releases) for previous versions.
