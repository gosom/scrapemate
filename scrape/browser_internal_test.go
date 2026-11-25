package scrape

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/mxschmitt/playwright-go"
	"github.com/stretchr/testify/require"
)

type cookieTestBrowser struct {
	playwright.Browser
	cdp playwright.CDPSession
}

func (b cookieTestBrowser) IsConnected() bool { return true }

func (b cookieTestBrowser) NewBrowserCDPSession() (playwright.CDPSession, error) {
	return b.cdp, nil
}

type cookieTestContext struct {
	playwright.BrowserContext
	clear func()
}

func (b cookieTestContext) IsClosed() bool { return false }

func (b cookieTestContext) ClearCookies(...playwright.BrowserContextClearCookiesOptions) error {
	b.clear()

	return nil
}

type cookieTestCDP struct {
	playwright.CDPSession
	send func(string, map[string]any) (any, error)
}

func (c cookieTestCDP) Send(method string, params map[string]any) (any, error) {
	return c.send(method, params)
}

func (c cookieTestCDP) Detach() error { return nil }

func cookieTestTargets(ids ...string) any {
	targets := make([]any, 0, len(ids))
	for _, id := range ids {
		targets = append(targets, map[string]any{"type": "page", "targetId": id})
	}

	return map[string]any{"targetInfos": targets}
}

func TestClearBrowserCookiesDrainsLatePopup(t *testing.T) {
	t.Parallel()

	active := "opener"

	var closed []string

	cdp := cookieTestCDP{send: func(method string, params map[string]any) (any, error) {
		switch method {
		case "Target.getTargets":
			if active == "" {
				return cookieTestTargets(), nil
			}

			return cookieTestTargets(active), nil
		case "Target.closeTarget":
			if params["targetId"] != active {
				return nil, fmt.Errorf("unexpected target ID %v", params["targetId"])
			}

			closed = append(closed, active)
			if active == "opener" {
				active = "late-popup"
			} else {
				active = ""
			}

			return nil, nil
		default:
			return nil, fmt.Errorf("unexpected command %s", method)
		}
	}}

	var cleared bool

	session := &browserSession{
		browser: cookieTestBrowser{cdp: cdp},
		context: cookieTestContext{clear: func() { cleared = active == "" }},
		opening: make(chan struct{}, 1),
	}

	require.NoError(t, session.clearCookies(t.Context()))
	require.Equal(t, []string{"opener", "late-popup"}, closed)
	require.True(t, cleared)
}

type cookieTestDriver struct {
	stopped chan struct{}
}

func (d cookieTestDriver) Pid() int { return 0 }

func (d cookieTestDriver) Stop() error {
	close(d.stopped)

	return nil
}

func TestClearBrowserCookiesInterruptsStalledCleanup(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name    string
		close   bool
		wantErr error
	}{
		{name: "caller cancels", wantErr: context.Canceled},
		{name: "client closes", close: true, wantErr: ErrClosed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			testStalledCookieCleanup(t, tc.close, tc.wantErr)
		})
	}
}

func testStalledCookieCleanup(t *testing.T, closeClient bool, wantErr error) {
	t.Helper()

	stalled := make(chan struct{})
	stopped := make(chan struct{})
	cdp := cookieTestCDP{send: func(method string, _ map[string]any) (any, error) {
		if method == "Target.getTargets" {
			return cookieTestTargets("opener"), nil
		}

		close(stalled)
		<-stopped

		return nil, playwright.ErrTargetClosed
	}}
	backend := &browserSession{
		browser: cookieTestBrowser{cdp: cdp},
		context: cookieTestContext{},
		driver:  cookieTestDriver{stopped: stopped},
		opening: make(chan struct{}, 1),
	}

	life, closeSession := context.WithCancel(t.Context())
	defer closeSession()

	client := &Client{session: backend, gate: make(chan struct{}, 1), life: life, cancel: closeSession, timeout: time.Second}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	cleared := make(chan error, 1)
	go func() { cleared <- client.ClearCookies(ctx) }()

	select {
	case <-stalled:
	case <-time.After(time.Second):
		t.Fatal("cleanup did not try to close the tab")
	}

	closed := make(chan error, 1)

	if closeClient {
		go func() { closed <- client.Close() }()
	} else {
		cancel()
	}

	select {
	case err := <-cleared:
		require.ErrorIs(t, err, wantErr)
	case <-time.After(time.Second):
		t.Fatal("stalled cleanup blocked ClearCookies")
	}

	if !closeClient {
		_, err := client.Fetch(t.Context(), "http://fixture.invalid")
		require.ErrorIs(t, err, ErrSessionLost)

		go func() { closed <- client.Close() }()
	}

	select {
	case err := <-closed:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("stalled cleanup blocked Close")
	}
}

type latePage struct {
	playwright.Page
	closed chan struct{}
}

func (p *latePage) Close(...playwright.PageCloseOptions) error {
	close(p.closed)

	return nil
}

func TestOpenPageClosesLateTab(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	opening := make(chan struct{}, 1)
	started := make(chan struct{})
	release := make(chan struct{})
	page := &latePage{closed: make(chan struct{})}
	done := make(chan error, 1)

	go func() {
		_, err := openPage(ctx, opening, func() (playwright.Page, error) {
			close(started)
			<-release

			return page, nil
		})
		done <- err
	}()

	<-started
	cancel()

	select {
	case err := <-done:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(time.Second):
		t.Error("tab creation blocked cancellation")
	}

	close(release)

	select {
	case <-page.closed:
	case <-time.After(time.Second):
		t.Fatal("late tab was not closed")
	}
}
