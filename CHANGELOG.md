# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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
