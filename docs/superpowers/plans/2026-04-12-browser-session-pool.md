# Browser Session Pool Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the eager Playwright browser pool with a lazy, bounded session-slot pool that preserves session state while reducing RAM growth, CPU overhead, and Chromium process count.

**Architecture:** Keep one durable browser context per slot so cookies and storage still persist, but create the browser lazily and reuse one primary page per slot. Clean each slot after every job, recycle only when unhealthy, and stop creating extra long-lived browsers beyond the intended concurrency.

**Tech Stack:** Go, Playwright Go, `scrapemateapp`, Go testing package

---

## File Structure

- Modify: `adapters/fetchers/jshttp/jshttp.go`
  Replace eager browser allocation and ad hoc overflow creation with a bounded lazy slot pool.
- Create: `adapters/fetchers/jshttp/session_slot.go`
  Encapsulate slot lifecycle, cleanup, page reuse, and recycling decisions.
- Create: `adapters/fetchers/jshttp/session_slot_test.go`
  Unit tests for lazy init, cleanup, page reuse, and recovery behavior using internal fakes.
- Modify: `scrapemateapp/jsfetcher_playwright.go`
  Keep the public wiring unchanged while documenting that concurrency now controls slot capacity instead of eager browser count.
- Modify: `scrapemateapp/config_test.go`
  Assert existing config behavior remains source-compatible.

### Task 1: Introduce Slot Lifecycle Test Seams

**Files:**
- Modify: `adapters/fetchers/jshttp/jshttp.go`
- Create: `adapters/fetchers/jshttp/session_slot_test.go`
- Test: `adapters/fetchers/jshttp/session_slot_test.go`

- [ ] **Step 1: Write the failing tests**

```go
package jshttp

import (
	"context"
	"testing"
)

func TestSessionSlotInitializeIsLazy(t *testing.T) {
	t.Parallel()

	factory := &fakeRuntimeFactory{}
	slot := newSessionSlot(factory)

	if factory.browserCreations != 0 {
		t.Fatalf("expected no browsers before first use, got %d", factory.browserCreations)
	}

	if err := slot.ensureReady(context.Background()); err != nil {
		t.Fatalf("ensureReady returned error: %v", err)
	}

	if factory.browserCreations != 1 {
		t.Fatalf("expected one browser creation after first use, got %d", factory.browserCreations)
	}
}

func TestSessionSlotCleanupLeavesSinglePrimaryPage(t *testing.T) {
	t.Parallel()

	slot := newSessionSlot(newFakeRuntimeFactoryWithPages(3))
	if err := slot.ensureReady(context.Background()); err != nil {
		t.Fatalf("ensureReady returned error: %v", err)
	}

	if err := slot.cleanup(context.Background()); err != nil {
		t.Fatalf("cleanup returned error: %v", err)
	}

	if got := slot.runtime.pageCount(); got != 1 {
		t.Fatalf("expected 1 page after cleanup, got %d", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./adapters/fetchers/jshttp -run 'TestSessionSlot'`

Expected: FAIL with undefined identifiers such as `newSessionSlot` and `fakeRuntimeFactory`.

- [ ] **Step 3: Write the minimal implementation scaffolding**

```go
package jshttp

import "context"

type slotRuntime interface {
	pageCount() int
	closeExtraPages() error
}

type runtimeFactory interface {
	create(context.Context) (slotRuntime, error)
}

type sessionSlot struct {
	factory runtimeFactory
	runtime slotRuntime
}

func newSessionSlot(factory runtimeFactory) *sessionSlot {
	return &sessionSlot{factory: factory}
}

func (s *sessionSlot) ensureReady(ctx context.Context) error {
	if s.runtime != nil {
		return nil
	}

	runtime, err := s.factory.create(ctx)
	if err != nil {
		return err
	}

	s.runtime = runtime
	return nil
}

func (s *sessionSlot) cleanup(_ context.Context) error {
	if s.runtime == nil {
		return nil
	}

	return s.runtime.closeExtraPages()
}
```

- [ ] **Step 4: Add the test fakes**

```go
package jshttp

import "context"

type fakeRuntimeFactory struct {
	browserCreations int
	pageTotal        int
}

func newFakeRuntimeFactoryWithPages(total int) *fakeRuntimeFactory {
	return &fakeRuntimeFactory{pageTotal: total}
}

func (f *fakeRuntimeFactory) create(context.Context) (slotRuntime, error) {
	f.browserCreations++
	total := f.pageTotal
	if total == 0 {
		total = 1
	}

	return &fakeRuntime{pages: total}, nil
}

type fakeRuntime struct {
	pages int
}

func (f *fakeRuntime) pageCount() int { return f.pages }

func (f *fakeRuntime) closeExtraPages() error {
	f.pages = 1
	return nil
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./adapters/fetchers/jshttp -run 'TestSessionSlot'`

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add adapters/fetchers/jshttp/jshttp.go adapters/fetchers/jshttp/session_slot_test.go
git commit -m "test: add session slot lifecycle seams"
```

### Task 2: Refactor JS Fetcher To Use A Bounded Lazy Slot Pool

**Files:**
- Modify: `adapters/fetchers/jshttp/jshttp.go`
- Create: `adapters/fetchers/jshttp/session_slot.go`
- Test: `adapters/fetchers/jshttp/session_slot_test.go`

- [ ] **Step 1: Write the failing pool-capacity test**

```go
func TestGetSlotWaitsAtCapacityInsteadOfCreatingOverflowBrowser(t *testing.T) {
	t.Parallel()

	fetcher := newJSFetchForTest(1)
	ctx := context.Background()

	first, err := fetcher.getSlot(ctx)
	if err != nil {
		t.Fatalf("getSlot returned error: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		_, err := fetcher.getSlot(ctx)
		done <- err
	}()

	select {
	case err := <-done:
		t.Fatalf("expected second acquisition to block, got %v", err)
	default:
	}

	fetcher.putSlot(ctx, first)

	if err := <-done; err != nil {
		t.Fatalf("expected blocked acquisition to resume cleanly, got %v", err)
	}

	if got := fetcher.factory.browserCreations; got != 1 {
		t.Fatalf("expected no overflow browser creation, got %d creations", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./adapters/fetchers/jshttp -run 'TestGetSlotWaitsAtCapacityInsteadOfCreatingOverflowBrowser'`

Expected: FAIL because `getSlot`, `putSlot`, or test helper wiring does not exist yet.

- [ ] **Step 3: Implement the lazy bounded slot pool**

```go
type jsFetch struct {
	pw                *playwright.Playwright
	headless          bool
	disableImages     bool
	pageReuseLimit    int
	browserReuseLimit int
	ua                string
	proxyPool         *ProxyPool

	slots chan *sessionSlot
}

func New(params JSFetcherOptions) (scrapemate.HTTPFetcher, error) {
	pw, err := playwright.Run()
	if err != nil {
		return nil, err
	}

	ans := &jsFetch{
		pw:                pw,
		headless:          params.Headless,
		disableImages:     params.DisableImages,
		pageReuseLimit:    params.PageReuseLimit,
		browserReuseLimit: params.BrowserReuseLimit,
		ua:                params.UserAgent,
		slots:             make(chan *sessionSlot, params.PoolSize),
	}

	for range params.PoolSize {
		ans.slots <- newSessionSlot(ans.newRuntimeFactory())
	}

	return ans, nil
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
```

- [ ] **Step 4: Update fetch path to use slots**

```go
func (o *jsFetch) Fetch(ctx context.Context, job scrapemate.IJob) scrapemate.Response {
	slot, err := o.getSlot(ctx)
	if err != nil {
		return scrapemate.Response{Error: err}
	}
	defer o.putSlot(ctx, slot)

	page, err := slot.acquirePage(ctx)
	if err != nil {
		return scrapemate.Response{Error: err}
	}

	resp := job.BrowserActions(ctx, playwrightadapter.NewPage(page))

	if cleanErr := slot.release(ctx); cleanErr != nil && resp.Error == nil {
		resp.Error = cleanErr
	}

	return resp
}
```

- [ ] **Step 5: Run focused tests**

Run: `go test ./adapters/fetchers/jshttp -run 'Test(SessionSlot|GetSlot)'`

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add adapters/fetchers/jshttp/jshttp.go adapters/fetchers/jshttp/session_slot.go adapters/fetchers/jshttp/session_slot_test.go
git commit -m "refactor: make js fetcher pool lazy and bounded"
```

### Task 3: Add Page Reuse, Cleanup, And Recovery Heuristics

**Files:**
- Modify: `adapters/fetchers/jshttp/session_slot.go`
- Modify: `adapters/fetchers/jshttp/session_slot_test.go`
- Test: `adapters/fetchers/jshttp/session_slot_test.go`

- [ ] **Step 1: Write the failing reuse and recovery tests**

```go
func TestSessionSlotReusesHealthyPrimaryPage(t *testing.T) {
	t.Parallel()

	slot := newSessionSlot(newFakeRuntimeFactoryWithPages(1))
	page1, err := slot.acquirePage(context.Background())
	if err != nil {
		t.Fatalf("acquirePage returned error: %v", err)
	}

	if err := slot.release(context.Background()); err != nil {
		t.Fatalf("release returned error: %v", err)
	}

	page2, err := slot.acquirePage(context.Background())
	if err != nil {
		t.Fatalf("acquirePage returned error: %v", err)
	}

	if page1 != page2 {
		t.Fatalf("expected primary page reuse")
	}
}

func TestSessionSlotRecreatesPageWhenPrimaryPageIsClosed(t *testing.T) {
	t.Parallel()

	factory := newFakeRuntimeFactoryWithPages(1)
	slot := newSessionSlot(factory)

	page1, err := slot.acquirePage(context.Background())
	if err != nil {
		t.Fatalf("acquirePage returned error: %v", err)
	}

	page1.closed = true

	page2, err := slot.acquirePage(context.Background())
	if err != nil {
		t.Fatalf("acquirePage returned error: %v", err)
	}

	if page1 == page2 {
		t.Fatalf("expected closed page to be replaced")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./adapters/fetchers/jshttp -run 'TestSessionSlot(ReusesHealthyPrimaryPage|RecreatesPageWhenPrimaryPageIsClosed)'`

Expected: FAIL because page reuse and recovery logic is not implemented yet.

- [ ] **Step 3: Implement slot acquire, release, and recovery**

```go
type sessionSlot struct {
	factory           runtimeFactory
	runtime           slotRuntime
	pageReuseLimit    int
	browserReuseLimit int
}

func (s *sessionSlot) acquirePage(ctx context.Context) (*fakePage, error) {
	if err := s.ensureReady(ctx); err != nil {
		return nil, err
	}

	page, err := s.runtime.primaryPage()
	if err == nil && !page.isClosed() {
		return page, nil
	}

	if err := s.runtime.recreatePage(); err != nil {
		if err := s.runtime.recreateContext(); err != nil {
			return nil, s.runtime.recreateBrowser()
		}
	}

	return s.runtime.primaryPage()
}

func (s *sessionSlot) release(ctx context.Context) error {
	if err := s.cleanup(ctx); err != nil {
		return err
	}

	return s.runtime.recycleIfNeeded()
}
```

- [ ] **Step 4: Extend the fake runtime to model healthy and closed pages**

```go
type fakePage struct {
	id     int
	closed bool
}

func (p *fakePage) isClosed() bool { return p.closed }

type fakeRuntime struct {
	pages      []*fakePage
	nextPageID int
}

func (f *fakeRuntime) primaryPage() (*fakePage, error) {
	if len(f.pages) == 0 {
		return nil, errNoPages
	}

	return f.pages[0], nil
}

func (f *fakeRuntime) recreatePage() error {
	f.nextPageID++
	f.pages = []*fakePage{{id: f.nextPageID}}
	return nil
}
```

- [ ] **Step 5: Run focused tests**

Run: `go test ./adapters/fetchers/jshttp -run 'TestSessionSlot'`

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add adapters/fetchers/jshttp/session_slot.go adapters/fetchers/jshttp/session_slot_test.go
git commit -m "feat: reuse and recycle js fetcher pages safely"
```

### Task 4: Preserve Public Wiring And Lock In Compatibility Tests

**Files:**
- Modify: `scrapemateapp/jsfetcher_playwright.go`
- Modify: `scrapemateapp/config_test.go`
- Modify: `scrapemateapp/config_compat_test.go`
- Test: `scrapemateapp/config_test.go`

- [ ] **Step 1: Write the failing compatibility test**

```go
func TestWithJSKeepsExistingFetcherConfigurationSurface(t *testing.T) {
	t.Parallel()

	writer := &mock.MockResultWriter{}

	cfg, err := scrapemateapp.NewConfig(
		[]scrapemate.ResultWriter{writer},
		scrapemateapp.WithConcurrency(4),
		scrapemateapp.WithBrowserReuseLimit(10),
		scrapemateapp.WithPageReuseLimit(5),
		scrapemateapp.WithJS(scrapemateapp.DisableImages()),
	)
	if err != nil {
		t.Fatalf("NewConfig returned error: %v", err)
	}

	if cfg.Concurrency != 4 || cfg.BrowserReuseLimit != 10 || cfg.PageReuseLimit != 5 || !cfg.UseJS {
		t.Fatalf("expected config surface to remain unchanged: %+v", cfg)
	}
}
```

- [ ] **Step 2: Run test to verify it fails or proves the gap**

Run: `go test ./scrapemateapp -run 'TestWithJSKeepsExistingFetcherConfigurationSurface'`

Expected: Either FAIL because the test does not exist yet, or PASS once added and used as a compatibility guard for the refactor.

- [ ] **Step 3: Update fetcher wiring comment and keep API behavior intact**

```go
func (app *ScrapemateApp) getJSFetcher(rotator scrapemate.ProxyRotator) (scrapemate.HTTPFetcher, error) {
	return jsfetcher.New(jsfetcher.JSFetcherOptions{
		Headless:          !app.cfg.JSOpts.Headfull,
		DisableImages:     app.cfg.JSOpts.DisableImages,
		Rotator:           rotator,
		PoolSize:          app.cfg.Concurrency,
		PageReuseLimit:    app.cfg.PageReuseLimit,
		BrowserReuseLimit: app.cfg.BrowserReuseLimit,
		UserAgent:         app.cfg.JSOpts.UA,
	})
}
```

- [ ] **Step 4: Run package tests**

Run: `go test ./scrapemateapp ./adapters/fetchers/jshttp`

Expected: PASS

- [ ] **Step 5: Run full test suite**

Run: `go test ./...`

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add scrapemateapp/jsfetcher_playwright.go scrapemateapp/config_test.go scrapemateapp/config_compat_test.go adapters/fetchers/jshttp/jshttp.go adapters/fetchers/jshttp/session_slot.go adapters/fetchers/jshttp/session_slot_test.go
git commit -m "feat: stabilize js fetcher resource reuse"
```

## Self-Review

- Spec coverage:
  The tasks cover lazy slot allocation, bounded pool behavior, page reuse, cleanup, recovery, and config compatibility. Multi-context-per-browser packing is intentionally not included because the spec excluded it from scope.
- Placeholder scan:
  No placeholder markers or cross-references without concrete content remain.
- Type consistency:
  The plan uses `sessionSlot`, `runtimeFactory`, `slotRuntime`, `getSlot`, and `putSlot` consistently across tasks. The actual implementation may refine fake-only types such as `fakePage`, but the production boundary remains the same.
