package scrape_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gosom/scrapemate/scrape"
	"github.com/stretchr/testify/require"
)

func TestResetSession(t *testing.T) {
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

			testResetSession(t, tc.js)
		})
	}
}

func testResetSession(t *testing.T, js bool) {
	t.Helper()

	newProxy := func(name string) string {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			w.Header().Set("X-Proxy", name)
			w.Header().Set("X-Received-Cookies", r.Header.Get("Cookie"))

			if r.URL.Path == "/seed" {
				w.Header().Set("Set-Cookie", "session=active; Path=/; HttpOnly")
				fmt.Fprint(w, `<script>localStorage.setItem("visitor", "returning")</script>`)

				return
			}

			fmt.Fprint(w, `<p id="visitor"></p><script>
document.querySelector("#visitor").textContent = localStorage.getItem("visitor") || "new";
</script>`)
		}))
		t.Cleanup(server.Close)

		return server.URL
	}

	opts := []scrape.Option{scrape.WithProxies([]string{newProxy("first"), newProxy("second")})}
	if js {
		opts = append(opts, scrape.WithJS())
	}

	client, err := scrape.New(opts...)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, client.Close()) })

	const host = "http://identity.fixture.invalid"

	seed := func() {
		t.Helper()
		_, fetchErr := client.Fetch(t.Context(), host+"/seed")
		require.NoError(t, fetchErr)
	}
	inspect := func(cookie, visitor string) string {
		t.Helper()
		page, fetchErr := client.Fetch(t.Context(), host+"/inspect")
		require.NoError(t, fetchErr)
		require.Equal(t, cookie, page.Response.Headers.Get("X-Received-Cookies"))

		if js {
			require.Contains(t, page.HTML(), `<p id="visitor">`+visitor+`</p>`)
		}

		return page.Response.Headers.Get("X-Proxy")
	}

	seed()

	previousProxy := inspect("session=active", "returning")
	require.NotEmpty(t, previousProxy)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	require.ErrorIs(t, client.ResetSession(ctx), context.Canceled)
	require.Equal(t, previousProxy, inspect("session=active", "returning"))

	require.NoError(t, client.ResetSession(t.Context()))

	newProxyName := inspect("", "new")
	require.NotEmpty(t, newProxyName)
	require.NotEqual(t, previousProxy, newProxyName)

	seed()
	require.Equal(t, newProxyName, inspect("session=active", "returning"))
	require.NoError(t, client.Close())
	require.ErrorIs(t, client.ResetSession(t.Context()), scrape.ErrClosed)
}
