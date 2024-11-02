package proxy //nolint:testpackage // need access to internal fields for testing

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewRotator(t *testing.T) {
	t.Run("creates rotator with valid proxies", func(t *testing.T) {
		proxies := []string{
			"socks5://proxy1.example.com:1080",
			"http://proxy2.example.com:8080",
			"https://proxy3.example.com:8080",
		}

		r := New(proxies)
		require.NotNil(t, r)
		require.Len(t, r.proxies, 3)
		require.Equal(t, "socks5://proxy1.example.com:1080", r.proxies[0].URL)
		require.Equal(t, "http://proxy2.example.com:8080", r.proxies[1].URL)
		require.Equal(t, "https://proxy3.example.com:8080", r.proxies[2].URL)
	})

	t.Run("panics with empty proxy list", func(t *testing.T) {
		require.Panics(t, func() {
			New([]string{})
		})
	})

	t.Run("panics with invalid proxy URL", func(t *testing.T) {
		require.Panics(t, func() {
			New([]string{"invalid://proxy"})
		})
	})
}

func TestRotatorNext(t *testing.T) {
	proxies := []string{
		"socks5://proxy1.example.com:1080",
		"http://proxy2.example.com:8080",
		"socks5://proxy3.example.com:1080",
		"https://proxy4.example.com:8080",
	}

	r := New(proxies)

	t.Run("rotates through proxies in order", func(t *testing.T) {
		p1 := r.Next()
		require.Equal(t, "socks5://proxy1.example.com:1080", p1.URL)

		p2 := r.Next()
		require.Equal(t, "http://proxy2.example.com:8080", p2.URL)

		p3 := r.Next()
		require.Equal(t, "socks5://proxy3.example.com:1080", p3.URL)

		p4 := r.Next()
		require.Equal(t, "https://proxy4.example.com:8080", p4.URL)

		p5 := r.Next()
		require.Equal(t, "socks5://proxy1.example.com:1080", p5.URL)
	})
}

func TestRotatorRoundTrip(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer testServer.Close()

	t.Run("creates and caches transport", func(t *testing.T) {
		proxies := []string{
			"http://proxy1.example.com:8080",
			"http://proxy2.example.com:8080",
		}
		r := New(proxies)

		req, err := http.NewRequest("GET", testServer.URL, http.NoBody)
		require.NoError(t, err)

		_, err = r.RoundTrip(req) //nolint:bodyclose // this is a test
		require.Error(t, err)

		transport, ok := r.cache.Load("http://proxy1.example.com:8080")
		require.True(t, ok)
		require.NotNil(t, transport)
	})

	t.Run("uses credentials when provided", func(t *testing.T) {
		proxies := []string{
			"http://proxy.example.com:8080",
		}
		r := New(proxies)
		r.proxies[0].Username = "user"
		r.proxies[0].Password = "pass"

		req, err := http.NewRequest("GET", testServer.URL, http.NoBody)
		require.NoError(t, err)

		_, err = r.RoundTrip(req) //nolint:bodyclose // this is a test
		require.Error(t, err)

		transport, ok := r.cache.Load("http://proxy.example.com:8080")
		require.True(t, ok)
		require.NotNil(t, transport)
	})

	t.Run("handles invalid proxy URL", func(t *testing.T) {
		proxies := []string{
			"http://proxy.example.com:8080",
		}
		r := New(proxies)
		r.proxies[0].URL = ":\\invalid"

		req, err := http.NewRequest("GET", testServer.URL, http.NoBody)
		require.NoError(t, err)

		_, err = r.RoundTrip(req) //nolint:bodyclose // this is a test
		require.Error(t, err)
		require.Contains(t, err.Error(), "error parsing proxy URL")
	})
}

func TestRotatorConcurrency(t *testing.T) {
	proxies := []string{
		"socks5://proxy1.example.com:1080",
		"http://proxy2.example.com:8080",
	}

	r := New(proxies)

	t.Run("handles concurrent access", func(t *testing.T) {
		var wg sync.WaitGroup

		iterations := 100

		seen := make(map[string]bool)

		var mu sync.Mutex

		for i := 0; i < iterations; i++ {
			wg.Add(1)

			go func() {
				defer wg.Done()

				proxy := r.Next()

				mu.Lock()
				seen[proxy.URL] = true
				mu.Unlock()
			}()
		}

		wg.Wait()

		require.Len(t, seen, 2)
		require.True(t, seen["socks5://proxy1.example.com:1080"])
		require.True(t, seen["http://proxy2.example.com:8080"])
	})
}
