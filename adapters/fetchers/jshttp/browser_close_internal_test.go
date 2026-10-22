package jshttp

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/mxschmitt/playwright-go"
	"github.com/stretchr/testify/require"
)

// hungContext is a BrowserContext whose Close blocks until released, like a
// context whose driver connection wedged.
type hungContext struct {
	playwright.BrowserContext
	release chan struct{}
}

func (c *hungContext) Close(...playwright.BrowserContextCloseOptions) error {
	<-c.release

	return nil
}

type closeCountingBrowser struct {
	playwright.Browser
	closed atomic.Int64
}

func (b *closeCountingBrowser) Close(...playwright.BrowserCloseOptions) error {
	b.closed.Add(1)

	return nil
}

func withBrowserCloseTimeout(t *testing.T, d time.Duration) {
	t.Helper()

	prev := browserCloseTimeout
	browserCloseTimeout = d

	t.Cleanup(func() { browserCloseTimeout = prev })
}

func TestBrowserCloseAbandonsHungContextAndStillClosesBrowser(t *testing.T) {
	withBrowserCloseTimeout(t, 100*time.Millisecond)

	ctx := &hungContext{release: make(chan struct{})}
	defer close(ctx.release)

	b := &closeCountingBrowser{}
	br := &browser{browser: b, ctx: ctx}

	done := make(chan struct{})

	go func() {
		br.Close()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("browser.Close blocked on a hung context close")
	}

	require.Equal(t, int64(1), b.closed.Load())
}

func TestPageSlotPoolCloseAbandonsHungContext(t *testing.T) {
	withBrowserCloseTimeout(t, 100*time.Millisecond)

	ctx := &hungContext{release: make(chan struct{})}
	defer close(ctx.release)

	b := &closeCountingBrowser{}
	pool := &pageSlotPool{slots: []*pageSlot{{browser: b, ctx: ctx}, {browser: b, ctx: ctx}}}

	done := make(chan struct{})

	go func() {
		pool.close()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("pageSlotPool.close blocked on a hung context close")
	}

	require.Equal(t, int64(2), b.closed.Load())
}
