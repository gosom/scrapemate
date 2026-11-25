package scrape

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"

	"github.com/gosom/scrapemate"
	browseradapter "github.com/gosom/scrapemate/adapters/browsers/playwright"
	"github.com/gosom/scrapemate/adapters/fetchers/jshttp"
	"github.com/mxschmitt/playwright-go"
)

// ErrSessionLost indicates that the browser lost the client's session.
var ErrSessionLost = errors.New("scrape: browser session lost")

type browserSession struct {
	driver  *playwright.Playwright
	browser playwright.Browser
	context playwright.BrowserContext
	bridge  *jshttp.AuthProxy
	opening chan struct{}
}

func newBrowserSession(endpoint *url.URL) (*browserSession, error) {
	driver, err := playwright.Run(&playwright.RunOptions{})
	if err != nil {
		return nil, fmt.Errorf("scrape: start Playwright (install the matching driver and Chromium first): %w", err)
	}

	s := &browserSession{driver: driver, opening: make(chan struct{}, 1)}

	proxy, err := s.configureProxy(endpoint)
	if err != nil {
		return nil, errors.Join(err, s.close())
	}

	s.browser, err = driver.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(true),
		Proxy:    proxy,
	})
	if err != nil {
		return nil, errors.Join(fmt.Errorf("scrape: launch Chromium: %w", err), s.close())
	}

	s.context, err = s.browser.NewContext()
	if err != nil {
		return nil, errors.Join(fmt.Errorf("scrape: create browser session: %w", err), s.close())
	}

	return s, nil
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
	if !s.browser.IsConnected() {
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
	var errs []error

	if s.context != nil {
		errs = append(errs, s.context.Close())
	}

	if s.browser != nil {
		errs = append(errs, s.browser.Close())
	}

	errs = append(errs, s.driver.Stop())

	if s.bridge != nil {
		errs = append(errs, s.bridge.Close())
	}

	return errors.Join(errs...)
}
