package scrape_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"

	"github.com/gosom/scrapemate/scrape"
	"github.com/stretchr/testify/require"
)

func TestMarkdownSnapshot(t *testing.T) {
	t.Parallel()

	fixture, err := os.ReadFile("testdata/article.html")
	require.NoError(t, err)

	for _, mode := range []string{"http", "chromium"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()

			var options []scrape.Option

			fetchOptions := []scrape.FetchOption{scrape.WithOutputs(scrape.Markdown())}

			if mode == "chromium" {
				if os.Getenv("SCRAPEMATE_TEST_BROWSER") == "" {
					t.Skip("set SCRAPEMATE_TEST_BROWSER=1 to run Chromium")
				}

				options = append(options, scrape.WithJS())
				fetchOptions = append(fetchOptions, scrape.WaitFor("#ready"))
			}

			var requests atomic.Int32

			mux := http.NewServeMux()
			mux.Handle("/start", http.RedirectHandler("/docs/page", http.StatusFound))
			mux.HandleFunc("/docs/page", func(w http.ResponseWriter, _ *http.Request) {
				requests.Add(1)
				w.Header().Set("Content-Type", "text/html")
				_, _ = w.Write(fixture)
			})
			server := httptest.NewServer(mux)
			t.Cleanup(server.Close)

			client, err := scrape.New(options...)
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, client.Close()) })

			result, err := client.Fetch(t.Context(), server.URL+"/start", fetchOptions...)
			require.NoError(t, err)
			require.EqualValues(t, 1, requests.Load())
			require.Equal(t, fixture, result.Response.Body)

			artifact, ok := result.Artifact("markdown")
			require.True(t, ok)
			require.Equal(t, "text/markdown", artifact.MediaType)

			md := string(artifact.Data)
			for _, want := range []string{"# Installing the client", "- Download the binary", "    println(\"hello\")", "[Go instructions](" + server.URL + "/guide/setup#go)", "| Command", "| fetch"} {
				require.Contains(t, md, want)
			}

			for _, skip := range []string{"Global navigation", "Page navigation", "Site footer", "setTimeout"} {
				require.NotContains(t, md, skip)
			}

			require.Contains(t, result.HTML(), "Site footer")

			if mode == "chromium" {
				require.Contains(t, md, "Browser content")
			} else {
				require.NotContains(t, md, "Browser content")
			}
		})
	}
}

func TestOutputErrors(t *testing.T) {
	t.Parallel()

	var requests atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, "{}")
	}))
	t.Cleanup(server.Close)

	client, err := scrape.New()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, client.Close()) })

	markdown := scrape.WithOutputs(scrape.Markdown())
	_, err = client.Fetch(t.Context(), server.URL, markdown, scrape.WithOutputs(scrape.Markdown()))
	require.ErrorContains(t, err, "duplicate output")
	require.Zero(t, requests.Load())

	result, err := client.Fetch(t.Context(), server.URL, markdown)
	require.ErrorContains(t, err, "produce markdown")
	require.Equal(t, "{}", string(result.Response.Body))
	_, ok := result.Artifact("markdown")
	require.False(t, ok)
	require.EqualValues(t, 1, requests.Load())
}
