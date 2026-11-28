package scrape

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/gosom/scrapemate"
	browseradapter "github.com/gosom/scrapemate/adapters/browsers/playwright"
	"github.com/gosom/scrapemate/adapters/fetchers/jshttp"
	"github.com/mxschmitt/playwright-go"
)

// ErrSessionLost indicates that the browser lost the client's session.
var ErrSessionLost = errors.New("scrape: browser session lost")

type playwrightDriver interface {
	Stop() error
	Pid() int
}

type browserSession struct {
	driver  playwrightDriver
	browser playwright.Browser
	context playwright.BrowserContext
	bridge  *jshttp.AuthProxy
	opening chan struct{}
	lost    bool
}

func newBrowserSession(ctx context.Context, endpoint *url.URL) (*browserSession, error) {
	driver, err := playwright.Run(&playwright.RunOptions{})
	if err != nil {
		return nil, fmt.Errorf("scrape: start Playwright (install the matching driver and Chromium first): %w", err)
	}

	s := &browserSession{driver: driver, opening: make(chan struct{}, 1)}

	stopped := make(chan error, 1)
	stop := context.AfterFunc(ctx, func() { stopped <- stopUnresponsiveDriver(driver) })

	err = s.launch(driver.Chromium, endpoint)
	if !stop() {
		return nil, errors.Join(ctx.Err(), <-stopped, s.closeProxy())
	}

	if err != nil {
		return nil, errors.Join(err, s.discard())
	}

	return s, nil
}

func (s *browserSession) launch(chromium playwright.BrowserType, endpoint *url.URL) error {
	proxy, err := s.configureProxy(endpoint)
	if err != nil {
		return err
	}

	s.browser, err = chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(true),
		Proxy:    proxy,
	})
	if err != nil {
		return fmt.Errorf("scrape: launch Chromium: %w", err)
	}

	s.context, err = s.browser.NewContext()
	if err != nil {
		return fmt.Errorf("scrape: create browser session: %w", err)
	}

	return nil
}

func (s *browserSession) configureProxy(endpoint *url.URL) (*playwright.Proxy, error) {
	if endpoint == nil {
		return nil, nil
	}

	server := *endpoint
	server.User = nil

	proxy := &playwright.Proxy{Server: server.String(), Bypass: playwright.String("<-loopback>")}
	if server.Scheme == "socks5" || server.Scheme == "socks5h" {
		if server.Port() == "" {
			server.Host = net.JoinHostPort(server.Hostname(), "1080")
		}

		if endpoint.User == nil {
			server.Scheme = "socks5"
			proxy.Server = server.String()

			return proxy, nil
		}

		password, _ := endpoint.User.Password()

		bridge, err := jshttp.StartAuthProxyPreservingRedirects(server.String(), endpoint.User.Username(), password)
		if err != nil {
			return nil, fmt.Errorf("scrape: start SOCKS proxy bridge: %w", err)
		}

		s.bridge = bridge
		proxy.Server = "http://" + bridge.Address()
	} else if endpoint.User != nil {
		password, _ := endpoint.User.Password()
		proxy.Username = playwright.String(endpoint.User.Username())
		proxy.Password = playwright.String(password)
	}

	return proxy, nil
}

func (s *browserSession) fetch(ctx context.Context, target string, cfg fetchConfig) (*Result, error) {
	if s.sessionLost() {
		return nil, ErrSessionLost
	}

	page, err := openPage(ctx, s.opening, s.context.NewPage)
	if err != nil {
		return nil, fmt.Errorf("open browser page: %w", err)
	}

	// Playwright calls do not take a Go context. Closing this fetch's tab
	// interrupts pending commands without clearing the account's context.
	closed := make(chan error, 1)
	stop := context.AfterFunc(ctx, func() { closed <- page.Close() })

	page.SetDefaultTimeout(0)

	result, err := readPage(page, target, cfg)

	var closeErr error

	if stop() {
		closeErr = page.Close()
	} else {
		closeErr = <-closed
	}

	if ctx.Err() != nil {
		err = ctx.Err()
	}

	if closeErr != nil {
		err = errors.Join(err, fmt.Errorf("close browser page: %w", closeErr))
	}

	return result, err
}

func (s *browserSession) clearCookies(ctx context.Context) error {
	if s.sessionLost() {
		return ErrSessionLost
	}

	// A canceled fetch can still have a tab opening in the background.
	select {
	case s.opening <- struct{}{}:
		defer func() { <-s.opening }()
	case <-ctx.Done():
		return ctx.Err()
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	// Playwright commands do not take a Go context. After cancellation, keep
	// this session unavailable and let Close stop the driver directly.
	done := make(chan error, 1)
	go func() { done <- s.clearBrowserCookies(ctx) }()

	select {
	case err := <-done:
		if err != nil {
			s.lost = true

			return fmt.Errorf("%w: %w", ErrSessionLost, err)
		}

		return nil
	case <-ctx.Done():
		s.lost = true

		return ctx.Err()
	}
}

func (s *browserSession) clearBrowserCookies(ctx context.Context) (resultErr error) {
	if err := ctx.Err(); err != nil {
		return err
	}

	cdp, err := s.browser.NewBrowserCDPSession()
	if err != nil {
		return fmt.Errorf("open Chromium protocol session: %w", err)
	}
	defer func() {
		if ctxErr := ctx.Err(); ctxErr != nil {
			resultErr = errors.Join(resultErr, ctxErr)

			return
		}

		if detachErr := cdp.Detach(); detachErr != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("detach Chromium protocol session: %w", detachErr))
		}
	}()

	if drainErr := drainBrowserTabs(ctx, cdp); drainErr != nil {
		return drainErr
	}

	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}

	if clearErr := s.context.ClearCookies(); clearErr != nil {
		return fmt.Errorf("clear browser cookies: %w", clearErr)
	}

	return nil
}

func drainBrowserTabs(ctx context.Context, cdp playwright.CDPSession) error {
	var failedTarget string

	var closeErr error

	for ctx.Err() == nil {
		targets, err := browserTabTargets(cdp)
		if err != nil {
			return err
		}

		if len(targets) == 0 {
			return nil
		}

		for _, target := range targets {
			if target == failedTarget {
				return fmt.Errorf("close Chromium tab: %w", closeErr)
			}
		}

		// Chromium reports popups before Playwright finishes initializing
		// them. Re-list targets after each close to find any late arrivals.
		target := targets[0]

		if err := ctx.Err(); err != nil {
			return err
		}

		if _, closeErr = cdp.Send("Target.closeTarget", map[string]any{"targetId": target}); closeErr != nil {
			// The tab may have closed itself since the previous listing.
			failedTarget = target
		}
	}

	return ctx.Err()
}

func browserTabTargets(cdp playwright.CDPSession) ([]string, error) {
	result, err := cdp.Send("Target.getTargets", map[string]any{})
	if err != nil {
		return nil, fmt.Errorf("list Chromium tabs: %w", err)
	}

	response, ok := result.(map[string]any)

	infos, listOK := response["targetInfos"].([]any)
	if !ok || !listOK {
		return nil, errors.New("invalid Chromium target list")
	}

	// The client owns a dedicated Chromium instance with one context.
	tabs := make([]string, 0, len(infos))
	for _, info := range infos {
		target, valid := info.(map[string]any)
		if !valid {
			return nil, errors.New("invalid Chromium target")
		}

		if target["type"] != "page" {
			continue
		}

		id, valid := target["targetId"].(string)
		if !valid || id == "" {
			return nil, errors.New("missing Chromium tab ID")
		}

		tabs = append(tabs, id)
	}

	return tabs, nil
}

func (s *browserSession) sessionLost() bool {
	return s.lost || !s.browser.IsConnected() || s.context.IsClosed()
}

// Keep at most one pending NewPage command. The unbuffered result transfers
// ownership to the caller only if it is still waiting.
func openPage(ctx context.Context, opening chan struct{}, newPage func() (playwright.Page, error)) (playwright.Page, error) {
	select {
	case opening <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	if err := ctx.Err(); err != nil {
		<-opening

		return nil, err
	}

	type outcome struct {
		page playwright.Page
		err  error
	}

	opened := make(chan outcome)

	go func() {
		defer func() { <-opening }()

		page, err := newPage()
		select {
		case opened <- outcome{page: page, err: err}:
		case <-ctx.Done():
			if page != nil {
				// The caller has returned. Closing the tab is best effort if
				// the browser session is being shut down at the same time.
				_ = page.Close()
			}
		}
	}()

	select {
	case result := <-opened:
		return result.page, result.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func readPage(page playwright.Page, target string, cfg fetchConfig) (*Result, error) {
	navigation, err := page.Goto(target, playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateDomcontentloaded})
	if err != nil {
		return nil, fmt.Errorf("navigate: %w", err)
	}

	if navigation == nil {
		return nil, errors.New("navigation did not produce an HTTP response")
	}

	result := &Result{
		Response: scrapemate.Response{
			URL:        navigation.URL(),
			StatusCode: navigation.Status(),
			Headers:    make(http.Header),
		},
		renderedHTML: new(string),
	}

	headers, err := navigation.HeadersArray()
	if err != nil {
		return result, fmt.Errorf("read navigation headers: %w", err)
	}

	for _, header := range headers {
		result.Response.Headers.Add(header.Name, header.Value)
	}

	result.Response.Body, err = navigation.Body()
	if err != nil {
		return result, fmt.Errorf("read navigation response: %w", err)
	}

	if !isHTML(&result.Response) {
		if cfg.selector != "" {
			return result, errors.New("wait for selector requires an HTML response")
		}

		return result, nil
	}

	adapted := browseradapter.NewPage(page)
	if cfg.selector != "" {
		if waitErr := adapted.WaitForSelector(cfg.selector, 0); waitErr != nil {
			return result, fmt.Errorf("wait for selector: %w", waitErr)
		}
	}

	*result.renderedHTML, err = adapted.Content()
	if err != nil {
		return result, fmt.Errorf("read rendered HTML: %w", err)
	}

	return result, nil
}

func (s *browserSession) close() error {
	if s.lost {
		return s.discard()
	}

	return errors.Join(s.context.Close(), s.browser.Close(), s.driver.Stop(), s.closeProxy())
}

func (s *browserSession) discard() error {
	// A retired session has its own driver. Closing the pipe stops all of
	// its browser activity, even if context or page commands are stalled.
	return errors.Join(stopUnresponsiveDriver(s.driver), s.closeProxy())
}

func (s *browserSession) closeProxy() error {
	if s.bridge != nil {
		return s.bridge.Close()
	}

	return nil
}

func stopUnresponsiveDriver(driver playwrightDriver) error {
	var process *os.Process

	var lookupErr error
	if pid := driver.Pid(); pid > 0 {
		process, lookupErr = os.FindProcess(pid)
		if process != nil {
			defer func() {
				// The Playwright driver reaps its process through Stop.
				_ = process.Release()
			}()
		}
	}

	stopped := make(chan error, 1)
	go func() { stopped <- driver.Stop() }()

	select {
	case err := <-stopped:
		return err
	case <-time.After(5 * time.Second):
		timeoutErr := errors.New("stop Playwright driver: timed out")
		if process == nil {
			return errors.Join(timeoutErr, lookupErr)
		}

		if err := process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
			return errors.Join(timeoutErr, err)
		}

		return timeoutErr
	}
}
