package scrape_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gosom/scrapemate/scrape"
	"github.com/stretchr/testify/require"
)

func TestFetchSession(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name  string
		js    bool
		socks bool
	}{
		{name: "http"},
		{name: "chromium", js: true},
		{name: "chromium-socks", js: true, socks: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if tc.js && os.Getenv("SCRAPEMATE_TEST_BROWSER") == "" {
				t.Skip("set SCRAPEMATE_TEST_BROWSER=1 with Playwright and Chromium installed")
			}

			testFetchSession(t, tc.js, tc.socks)
		})
	}
}

func testFetchSession(t *testing.T, js, socks bool) {
	t.Helper()

	var proxied atomic.Int64

	fixture := http.NewServeMux()
	fixture.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{ //nolint:gosec // the local proxy fixture serves plain HTTP.
			Name: "session", Value: "signed-in", Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode,
		})
		http.Redirect(w, r, "/account", http.StatusFound)
	})
	fixture.HandleFunc("/account", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Add("Set-Cookie", "first=1; Path=/")
		w.Header().Add("Set-Cookie", "second=2; Path=/")

		if cookie, err := r.Cookie("session"); err == nil && cookie.Value == "signed-in" {
			fmt.Fprint(w, `<p>signed-in</p><script>setTimeout(() => {
  const element = document.createElement("div");
  element.id = "ready";
  element.textContent = "Rendered";
  document.body.append(element);
}, 200);</script>`)

			return
		}

		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, "<p>signed-out</p>")
	})
	fixture.HandleFunc("/missing", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Fixture", "missing")
		http.Error(w, "missing page", http.StatusNotFound)
	})
	fixture.HandleFunc("/slow", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "<p>loading</p>")

		if err := http.NewResponseController(w).Flush(); err != nil {
			return
		}

		<-r.Context().Done()
	})

	endpoint := sessionProxy(t, fixture, &proxied, socks)
	options := []scrape.Option{scrape.WithProxies([]string{endpoint})}

	var fetchOptions []scrape.FetchOption

	if js {
		options = append(options, scrape.WithJS())
		fetchOptions = append(fetchOptions, scrape.WaitFor("#ready"))
	}

	client, err := scrape.New(options...)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, client.Close()) })

	const base = "http://fixture.invalid"

	result, err := client.Fetch(t.Context(), base+"/login", fetchOptions...)
	require.NoError(t, err)
	require.Equal(t, base+"/account", result.Response.URL)
	require.Contains(t, result.HTML(), "<p>signed-in</p>")
	require.NotContains(t, string(result.Response.Body), `<div id="ready">`)
	require.ElementsMatch(t, []string{"first=1; Path=/", "second=2; Path=/"}, result.Response.Headers.Values("Set-Cookie"))

	if js {
		require.Contains(t, result.HTML(), `<div id="ready">Rendered</div>`)
	}

	checkSession(t, client, options, base)
	require.GreaterOrEqual(t, proxied.Load(), int64(5))
}

func sessionProxy(t *testing.T, fixture http.Handler, proxied *atomic.Int64, socks bool) string {
	t.Helper()

	if socks {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			proxied.Add(1)
			fixture.ServeHTTP(w, r)
		}))
		t.Cleanup(server.Close)

		return authenticatedSOCKSProxy(t, server.Listener.Addr().String())
	}

	// The target host only exists behind this test proxy.
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Host != "fixture.invalid" {
			w.WriteHeader(http.StatusBadGateway)

			return
		}

		if r.Header.Get("Proxy-Authorization") != "Basic dXNlcjpwYXNzd29yZA==" {
			w.Header().Set("Proxy-Authenticate", `Basic realm="fixture"`)
			w.WriteHeader(http.StatusProxyAuthRequired)

			return
		}

		proxied.Add(1)
		fixture.ServeHTTP(w, r)
	}))
	t.Cleanup(proxy.Close)

	endpoint, err := url.Parse(proxy.URL)
	require.NoError(t, err)

	endpoint.User = url.UserPassword("user", "password")

	return endpoint.String()
}

func checkSession(t *testing.T, client *scrape.Client, options []scrape.Option, base string) {
	t.Helper()

	result, err := client.Fetch(t.Context(), base+"/account")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, result.Response.StatusCode)

	isolated, err := scrape.New(options...)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, isolated.Close()) })

	result, err = isolated.Fetch(t.Context(), base+"/account")
	require.NoError(t, err)
	require.Equal(t, http.StatusUnauthorized, result.Response.StatusCode)

	result, err = client.Fetch(t.Context(), base+"/missing")
	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound, result.Response.StatusCode)
	require.Equal(t, "missing", result.Response.Headers.Get("X-Fixture"))
	require.Contains(t, string(result.Response.Body), "missing page")
	require.Empty(t, result.HTML())

	timeoutCtx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()

	_, err = client.Fetch(timeoutCtx, base+"/slow")
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.NotContains(t, err.Error(), "password")

	result, err = client.Fetch(t.Context(), base+"/account")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, result.Response.StatusCode)
	require.NoError(t, client.Close())
	require.NoError(t, client.Close())

	_, err = client.Fetch(t.Context(), base+"/account")
	require.ErrorIs(t, err, scrape.ErrClosed)
}

func TestCloseCancelsFetch(t *testing.T) {
	t.Parallel()

	started := make(chan struct{})

	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		close(started)
		<-r.Context().Done()
	}))
	defer server.Close()

	client, err := scrape.New()
	require.NoError(t, err)

	done := make(chan error, 1)

	go func() {
		_, fetchErr := client.Fetch(t.Context(), server.URL)
		done <- fetchErr
	}()

	select {
	case <-started:
	case fetchErr := <-done:
		t.Fatalf("fetch returned before reaching server: %v", fetchErr)
	case <-time.After(5 * time.Second):
		t.Fatal("fetch did not reach server")
	}

	require.NoError(t, client.Close())
	require.ErrorIs(t, <-done, scrape.ErrClosed)
}

func TestProxyURLPath(t *testing.T) {
	t.Parallel()

	var proxied atomic.Int64

	endpoint := sessionProxy(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, "through proxy")
	}), &proxied, false)

	for _, tc := range []struct {
		name, path string
		valid      bool
	}{
		{"no path", "", true},
		{"root", "/", true},
		{"non-root", "/api", false},
		{"repeated slash", "//", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client, err := scrape.New(scrape.WithProxies([]string{endpoint + tc.path}))
			if !tc.valid {
				require.ErrorContains(t, err, "invalid proxy")

				return
			}

			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, client.Close()) })

			page, err := client.Fetch(t.Context(), "http://fixture.invalid/")
			require.NoError(t, err)
			require.Equal(t, "through proxy", string(page.Response.Body))
		})
	}
}

func TestInvalidConfiguration(t *testing.T) {
	t.Parallel()

	_, err := scrape.New(scrape.WithProxies([]string{"http://user:sensitive@%"}))
	require.Error(t, err)
	require.False(t, strings.Contains(err.Error(), "sensitive"))

	client, err := scrape.New()

	require.NoError(t, err)
	defer func() { require.NoError(t, client.Close()) }()

	result, err := client.Fetch(t.Context(), "http://fixture.invalid", scrape.WaitFor("body"))
	require.ErrorContains(t, err, "requires JavaScript")
	require.Nil(t, result)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, err = client.Fetch(ctx, "http://fixture.invalid")
	require.True(t, errors.Is(err, context.Canceled))
}
