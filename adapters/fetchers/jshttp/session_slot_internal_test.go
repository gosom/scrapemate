package jshttp

import (
	"context"
	"testing"
)

func newJSFetchForTest(size int) (*jsFetch, *fakeRuntimeFactory) {
	factory := &fakeRuntimeFactory{}
	slots := make(chan *sessionSlot, size)

	for range size {
		slots <- newSessionSlot(factory)
	}

	return &jsFetch{
		slots: slots,
	}, factory
}

func TestGetSlotWaitsAtCapacityInsteadOfCreatingOverflowBrowser(t *testing.T) {
	t.Parallel()

	fetcher, factory := newJSFetchForTest(1)
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

	if got := factory.browserCreations; got > 0 {
		t.Fatalf("expected no browsers before slot use, got %d creations", got)
	}
}

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

type fakePage struct {
	id     int
	closed bool
}

func (p *fakePage) isClosed() bool { return p.closed }

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

	pages := make([]*fakePage, total)
	for i := range pages {
		pages[i] = &fakePage{id: i + 1}
	}

	return &fakeRuntime{pages: pages, nextPageID: total + 1}, nil
}

type fakeRuntime struct {
	pages      []*fakePage
	nextPageID int
}

func (f *fakeRuntime) pageCount() int { return len(f.pages) }

func (f *fakeRuntime) closeExtraPages() error {
	if len(f.pages) > 1 {
		f.pages = f.pages[:1]
	}

	return nil
}

func (f *fakeRuntime) closeBrowser() error { return nil }

func (f *fakeRuntime) primaryPage() (page, error) {
	if len(f.pages) == 0 {
		return nil, errNoPages
	}

	return f.pages[0], nil
}

func (f *fakeRuntime) recreatePage() error {
	p := &fakePage{id: f.nextPageID}
	f.nextPageID++
	f.pages = []*fakePage{p}

	return nil
}

func (f *fakeRuntime) recreateContext() error {
	return f.recreatePage()
}

func (f *fakeRuntime) recreateBrowser() error {
	return f.recreatePage()
}

func (f *fakeRuntime) recycleIfNeeded() error {
	return nil
}

func TestSessionSlotReusesHealthyPrimaryPage(t *testing.T) {
	t.Parallel()

	slot := newSessionSlot(newFakeRuntimeFactoryWithPages(1))

	page1, err := slot.acquirePage(context.Background())
	if err != nil {
		t.Fatalf("acquirePage returned error: %v", err)
	}

	if rerr := slot.release(context.Background()); rerr != nil {
		t.Fatalf("release returned error: %v", rerr)
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

	fp, ok := page1.(*fakePage)
	if !ok {
		t.Fatalf("expected *fakePage type")
	}

	fp.closed = true

	page2, err := slot.acquirePage(context.Background())
	if err != nil {
		t.Fatalf("acquirePage returned error: %v", err)
	}

	if page1 == page2 {
		t.Fatalf("expected closed page to be replaced")
	}
}
