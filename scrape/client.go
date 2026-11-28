// Package scrape fetches individual pages with reusable HTTP and browser sessions.
package scrape

import (
	"context"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gosom/scrapemate"
	"github.com/gosom/scrapemate/adapters/fetchers/nethttp"
	"golang.org/x/net/publicsuffix"
)

const defaultTimeout = 30 * time.Second

// ErrClosed indicates that the client has been closed.
var ErrClosed = errors.New("scrape: client is closed")

// Result holds the original navigation response and the page's HTML representation.
type Result struct {
	Response scrapemate.Response

	renderedHTML *string
}

// HTML returns the source HTML in HTTP mode or the rendered DOM in JavaScript
// mode. It returns an empty string for non-HTML responses or unavailable HTML.
func (r *Result) HTML() string {
	if r.renderedHTML != nil {
		return *r.renderedHTML
	}

	if isHTML(&r.Response) {
		return string(r.Response.Body)
	}

	return ""
}

func isHTML(resp *scrapemate.Response) bool {
	contentType := resp.Headers.Get("Content-Type")
	if contentType == "" {
		contentType = http.DetectContentType(resp.Body)
	}

	mediaType, _, _ := mime.ParseMediaType(contentType)

	return mediaType == "text/html" || mediaType == "application/xhtml+xml"
}

type clientConfig struct {
	js      bool
	timeout time.Duration
	proxies []string
}

type fetchConfig struct {
	selector string
}

// Option configures a client.
type Option func(*clientConfig) error

// FetchOption configures a single fetch.
type FetchOption func(*fetchConfig) error

// WithJS uses a headless Chromium session. Install the matching Playwright
// driver and Chromium browser before creating the client.
func WithJS() Option {
	return func(cfg *clientConfig) error {
		cfg.js = true

		return nil
	}
}

// WithTimeout sets the time budget for a fetch, including waiting for the
// client to become available. The default is 30 seconds.
func WithTimeout(timeout time.Duration) Option {
	return func(cfg *clientConfig) error {
		if timeout <= 0 {
			return errors.New("scrape: timeout must be positive")
		}

		cfg.timeout = timeout

		return nil
	}
}

// WithProxies supplies proxy endpoints. New clients select successive endpoints
// from the list. ResetSession advances the client's selection, and fetches keep
// using the selected endpoint between resets.
func WithProxies(proxies []string) Option {
	return func(cfg *clientConfig) error {
		cfg.proxies = append([]string(nil), proxies...)

		return nil
	}
}

// WaitFor waits for a visible CSS selector in JavaScript mode before taking
// the HTML snapshot.
func WaitFor(selector string) FetchOption {
	return func(cfg *fetchConfig) error {
		if strings.TrimSpace(selector) == "" {
			return errors.New("scrape: wait selector must not be empty")
		}

		cfg.selector = selector

		return nil
	}
}

type session interface {
	fetch(context.Context, string, fetchConfig) (*Result, error)
	clearCookies(context.Context) error
	close() error
	discard() error
}

// Client owns a private cookie session. It is safe for concurrent use and
// admits one fetch at a time. Create a client with New.
type Client struct {
	session    session
	newSession func(context.Context, *url.URL) (session, error)
	proxies    []*url.URL
	proxyIndex uint64
	timeout    time.Duration
	js         bool
	gate       chan struct{}
	replacing  chan struct{}
	life       context.Context
	cancel     context.CancelFunc
	once       sync.Once
	closeErr   error
}

var nextProxy atomic.Uint64

// New creates a client. By default it uses HTTP and needs no browser installation.
func New(opts ...Option) (*Client, error) {
	cfg := clientConfig{timeout: defaultTimeout}

	for _, opt := range opts {
		if err := opt(&cfg); err != nil {
			return nil, err
		}
	}

	proxies, err := parseProxies(cfg.proxies)
	if err != nil {
		return nil, err
	}

	newSession := func(ctx context.Context, endpoint *url.URL) (session, error) {
		if cfg.js {
			return newBrowserSession(ctx, endpoint)
		}

		return newHTTPSession(endpoint)
	}

	var endpoint *url.URL

	var proxyIndex uint64
	if len(proxies) > 0 {
		proxyIndex = (nextProxy.Add(1) - 1) % uint64(len(proxies))
		endpoint = proxies[proxyIndex]
	}

	backend, err := newSession(context.Background(), endpoint)
	if err != nil {
		return nil, err
	}

	life, cancel := context.WithCancel(context.Background())

	return &Client{
		session:    backend,
		newSession: newSession,
		proxies:    proxies,
		proxyIndex: proxyIndex,
		timeout:    cfg.timeout,
		js:         cfg.js,
		gate:       make(chan struct{}, 1),
		replacing:  make(chan struct{}, 1),
		life:       life,
		cancel:     cancel,
	}, nil
}

// Fetch performs a GET. HTTP status codes, including non-2xx codes, are
// returned in the result. If a later step fails, Fetch returns any available
// response alongside the error.
// For downloads, use HTTP mode. JavaScript mode returns a navigation error
// when Chromium treats a response as a download.
func (c *Client) Fetch(ctx context.Context, target string, opts ...FetchOption) (*Result, error) {
	if c.life.Err() != nil {
		return nil, ErrClosed
	}

	cfg, err := c.validateFetch(target, opts)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	stop := context.AfterFunc(c.life, cancel)
	defer stop()

	if err = c.acquire(ctx); err != nil {
		return nil, c.fetchError(err)
	}
	defer func() { <-c.gate }()

	started := time.Now()
	result, err := c.session.fetch(ctx, target, cfg)

	if result != nil {
		result.Response.Duration = time.Since(started)
	}

	if err != nil {
		return result, c.fetchError(err)
	}

	return result, nil
}

// ClearCookies removes all cookies from the client's session across all hosts.
// It leaves the proxy and browser local storage in place. It waits for the
// current fetch to finish. If ctx expires during a browser command, the
// browser session becomes unusable and fetches and clears return ErrSessionLost
// until the session is reset.
// Sites can set fresh cookies through subsequent requests, including requests
// from browser service workers.
func (c *Client) ClearCookies(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	stop := context.AfterFunc(c.life, cancel)
	defer stop()

	if err := c.acquire(ctx); err != nil {
		return c.clearCookiesError(err)
	}
	defer func() { <-c.gate }()

	if err := c.session.clearCookies(ctx); err != nil {
		return c.clearCookiesError(err)
	}

	return nil
}

// ResetSession replaces the client's session. It starts with a fresh cookie
// jar in HTTP mode or fresh browser storage in JavaScript mode, and selects
// the next configured proxy. It waits for any active fetch before replacing
// the session, and can recover a lost browser session.
//
// Cancellation before replacement leaves the previous session in place.
// If cancellation or a cleanup error occurs while closing the previous
// session, the new session remains active.
func (c *Client) ResetSession(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	stop := context.AfterFunc(c.life, cancel)
	defer stop()

	err := c.resetSession(ctx)
	if err == nil {
		return nil
	}

	if c.life.Err() != nil {
		return ErrClosed
	}

	return fmt.Errorf("scrape: reset session: %w", err)
}

func (c *Client) resetSession(ctx context.Context) error {
	if err := c.acquire(ctx); err != nil {
		return err
	}
	defer func() { <-c.gate }()

	// A canceled reset can still be unwinding a Playwright call. Allow only
	// one replacement to own resources at a time.
	select {
	case c.replacing <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}

	if err := ctx.Err(); err != nil {
		<-c.replacing

		return err
	}

	var endpoint *url.URL

	proxyIndex := c.proxyIndex
	if len(c.proxies) > 0 {
		proxyIndex = (proxyIndex + 1) % uint64(len(c.proxies))
		endpoint = c.proxies[proxyIndex]
	}

	type replacement struct {
		session session
		err     error
	}

	ready := make(chan replacement)
	discard := make(chan session, 1)
	cleaned := make(chan error, 1)

	go func() {
		defer func() { <-c.replacing }()

		fresh, err := c.newSession(ctx, endpoint)
		select {
		case ready <- replacement{session: fresh, err: err}:
			if err == nil {
				cleaned <- (<-discard).discard()
			}
		case <-ctx.Done():
			if err == nil {
				// A canceled caller can no longer take ownership.
				cleaned <- fresh.discard()
			}
		}
	}()

	select {
	case result := <-ready:
		if result.err != nil {
			return result.err
		}

		if err := ctx.Err(); err != nil {
			discard <- result.session

			return err
		}

		previous := c.session

		c.session = result.session
		c.proxyIndex = proxyIndex

		discard <- previous
	case <-ctx.Done():
		return ctx.Err()
	}

	select {
	case err := <-cleaned:
		if err != nil {
			return fmt.Errorf("discard previous session: %w", err)
		}

		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *Client) acquire(ctx context.Context) error {
	select {
	case c.gate <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	case <-c.life.Done():
		return ErrClosed
	}

	if c.life.Err() != nil {
		<-c.gate

		return ErrClosed
	}

	if err := ctx.Err(); err != nil {
		<-c.gate

		return err
	}

	return nil
}

func (c *Client) clearCookiesError(err error) error {
	if c.life.Err() != nil {
		return ErrClosed
	}

	return fmt.Errorf("scrape: clear cookies: %w", err)
}

func (c *Client) validateFetch(target string, opts []FetchOption) (fetchConfig, error) {
	var cfg fetchConfig

	parsed, err := url.Parse(target)
	if err != nil || parsed.Hostname() == "" || parsed.User != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return cfg, errors.New("scrape: expected an absolute HTTP(S) URL without credentials")
	}

	for _, opt := range opts {
		if err := opt(&cfg); err != nil {
			return cfg, err
		}
	}

	if cfg.selector != "" && !c.js {
		return cfg, errors.New("scrape: WaitFor requires JavaScript mode")
	}

	return cfg, nil
}

func (c *Client) fetchError(err error) error {
	if c.life.Err() != nil {
		return ErrClosed
	}

	return fmt.Errorf("scrape: fetch: %w", err)
}

// Close cancels active and waiting operations and releases the current
// session. Calling Close again returns the same result.
func (c *Client) Close() error {
	c.once.Do(func() {
		c.cancel()

		c.gate <- struct{}{}
		defer func() { <-c.gate }()

		c.closeErr = c.session.close()
	})

	return c.closeErr
}

type httpSession struct {
	fetcher scrapemate.HTTPFetcher
	client  *http.Client
}

func newHTTPSession(endpoint *url.URL) (*httpSession, error) {
	jar, err := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	if err != nil {
		return nil, fmt.Errorf("scrape: create cookie jar: %w", err)
	}

	// Each session owns its idle connections, even if the caller replaced
	// the process-wide default with a custom RoundTripper.
	transport := &http.Transport{Proxy: http.ProxyFromEnvironment}
	if standard, ok := http.DefaultTransport.(*http.Transport); ok {
		transport = standard.Clone()
	}

	if endpoint != nil {
		transport.Proxy = http.ProxyURL(endpoint)
	}

	client := &http.Client{Jar: jar, Transport: transport}

	return &httpSession{fetcher: nethttp.New(client), client: client}, nil
}

func (s *httpSession) fetch(ctx context.Context, target string, _ fetchConfig) (*Result, error) {
	resp := s.fetcher.Fetch(ctx, &scrapemate.Job{Method: http.MethodGet, URL: target})
	if resp.StatusCode == 0 {
		return nil, resp.Error
	}

	return &Result{Response: resp}, resp.Error
}

func (s *httpSession) clearCookies(ctx context.Context) error {
	jar, err := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	if err != nil {
		return fmt.Errorf("create cookie jar: %w", err)
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	// The client gate excludes HTTP requests while swapping the jar.
	s.client.Jar = jar

	return nil
}

func (s *httpSession) close() error {
	s.client.CloseIdleConnections()

	return s.fetcher.Close()
}

func (s *httpSession) discard() error {
	return s.close()
}

func parseProxies(endpoints []string) ([]*url.URL, error) {
	proxies := make([]*url.URL, 0, len(endpoints))
	for i, endpoint := range endpoints {
		if !strings.Contains(endpoint, "://") {
			endpoint = "socks5://" + endpoint
		}

		parsed, err := url.Parse(endpoint)
		if err != nil || parsed.Hostname() == "" || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
			return nil, fmt.Errorf("scrape: invalid proxy at position %d", i+1)
		}

		if _, err := scrapemate.NewProxy(endpoint); err != nil {
			return nil, fmt.Errorf("scrape: unsupported proxy at position %d", i+1)
		}

		proxies = append(proxies, parsed)
	}

	return proxies, nil
}
