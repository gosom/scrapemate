package scrape_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gosom/scrapemate/scrape"
	"github.com/stretchr/testify/require"
)

func TestClearCookies(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		js   bool
	}{
		{name: "http"},
		{name: "chromium", js: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if tc.js && os.Getenv("SCRAPEMATE_TEST_BROWSER") == "" {
				t.Skip("set SCRAPEMATE_TEST_BROWSER=1 with Playwright and Chromium installed")
			}

			testClearCookies(t, tc.js)
		})
	}
}

func testClearCookies(t *testing.T, js bool) {
	t.Helper()

	const (
		firstHost  = "http://first.fixture.invalid"
		secondHost = "http://second.fixture.invalid"
	)

	// Both fixture hosts are reachable only through the configured proxy.
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Header().Set("X-Received-Cookies", r.Header.Get("Cookie"))

		if r.URL.Path == "/seed" {
			w.Header().Add("Set-Cookie", "session=active; Path=/; HttpOnly")
			fmt.Fprint(w, `<script>localStorage.setItem("preference", "dark")</script>`)

			return
		}

		fmt.Fprint(w, `<p id="preference"></p><script>
document.querySelector("#preference").textContent = localStorage.getItem("preference");
</script>`)
	}))
	t.Cleanup(proxy.Close)

	options := []scrape.Option{scrape.WithProxies([]string{proxy.URL})}
	if js {
		options = append(options, scrape.WithJS())
	}

	client, err := scrape.New(options...)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, client.Close()) })

	for _, host := range []string{firstHost, secondHost} {
		_, fetchErr := client.Fetch(t.Context(), host+"/seed")
		require.NoError(t, fetchErr)
	}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	require.ErrorIs(t, client.ClearCookies(ctx), context.Canceled)

	for _, host := range []string{firstHost, secondHost} {
		page, fetchErr := client.Fetch(t.Context(), host+"/inspect")
		require.NoError(t, fetchErr)
		require.Equal(t, "session=active", page.Response.Headers.Get("X-Received-Cookies"))
	}

	require.NoError(t, client.ClearCookies(t.Context()))
	require.NoError(t, client.ClearCookies(t.Context()))

	for _, host := range []string{firstHost, secondHost} {
		page, fetchErr := client.Fetch(t.Context(), host+"/inspect")
		require.NoError(t, fetchErr)
		require.Empty(t, page.Response.Headers.Get("X-Received-Cookies"))

		if js {
			require.Contains(t, page.HTML(), `<p id="preference">dark</p>`)
		}
	}

	_, err = client.Fetch(t.Context(), firstHost+"/seed")
	require.NoError(t, err)

	page, err := client.Fetch(t.Context(), firstHost+"/inspect")
	require.NoError(t, err)
	require.Equal(t, "session=active", page.Response.Headers.Get("X-Received-Cookies"))
	require.NoError(t, client.Close())
	require.ErrorIs(t, client.ClearCookies(t.Context()), scrape.ErrClosed)
}

func TestClearCookiesClosesNavigatingPopup(t *testing.T) {
	t.Parallel()

	if os.Getenv("SCRAPEMATE_TEST_BROWSER") == "" {
		t.Skip("set SCRAPEMATE_TEST_BROWSER=1 with Playwright and Chromium installed")
	}

	started := make(chan struct{})
	canceled := make(chan struct{})
	fixture := http.NewServeMux()
	fixture.HandleFunc("/open", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<script>window.open("/popup")</script>`)
	})
	fixture.HandleFunc("/popup", func(_ http.ResponseWriter, r *http.Request) {
		close(started)
		<-r.Context().Done()
		close(canceled)
	})

	server := httptest.NewServer(fixture)
	t.Cleanup(server.Close)

	client, err := scrape.New(scrape.WithJS())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, client.Close()) })

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	_, err = client.Fetch(ctx, server.URL+"/open")
	require.NoError(t, err)

	select {
	case <-started:
	case <-ctx.Done():
		t.Fatal("popup navigation did not reach the server")
	}

	require.NoError(t, client.ClearCookies(ctx))

	select {
	case <-canceled:
	case <-ctx.Done():
		t.Fatal("popup navigation survived cookie clearing")
	}
}

func TestClearCookiesWhileFetching(t *testing.T) {
	t.Parallel()

	started := make(chan struct{})
	reply := make(chan struct{})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/slow" {
			close(started)

			select {
			case <-reply:
			case <-r.Context().Done():
				return
			}
		}

		if r.URL.Path == "/slow" || r.URL.Path == "/seed" {
			w.Header().Add("Set-Cookie", "session="+r.URL.Path[1:]+"; Path=/")
		}

		w.Header().Set("X-Received-Cookies", r.Header.Get("Cookie"))
	}))
	t.Cleanup(server.Close)

	client, err := scrape.New()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, client.Close()) })

	_, err = client.Fetch(t.Context(), server.URL+"/seed")
	require.NoError(t, err)

	fetched := make(chan error, 1)

	go func() {
		_, fetchErr := client.Fetch(t.Context(), server.URL+"/slow")
		fetched <- fetchErr
	}()

	select {
	case <-started:
	case fetchErr := <-fetched:
		t.Fatalf("fetch returned before reaching server: %v", fetchErr)
	case <-time.After(5 * time.Second):
		t.Fatal("fetch did not reach server")
	}

	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()

	require.ErrorIs(t, client.ClearCookies(ctx), context.DeadlineExceeded)

	cleared := make(chan error, 1)
	go func() { cleared <- client.ClearCookies(t.Context()) }()

	select {
	case clearErr := <-cleared:
		t.Fatalf("clear returned before the fetch finished: %v", clearErr)
	case <-time.After(50 * time.Millisecond):
	}

	close(reply)
	require.NoError(t, <-fetched)
	require.NoError(t, <-cleared)

	page, err := client.Fetch(t.Context(), server.URL+"/inspect")
	require.NoError(t, err)
	require.Empty(t, page.Response.Headers.Get("X-Received-Cookies"))
}
