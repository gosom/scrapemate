package main

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFetchCommand(t *testing.T) {
	t.Setenv("SCRAPEMATE_PROXY", "")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")

		if r.URL.Path == "/missing" {
			w.WriteHeader(http.StatusNotFound)
		}

		fmt.Fprint(w, "<main><h1>Reading</h1></main><footer>Contact</footer>")
	}))
	t.Cleanup(server.Close)

	for _, tc := range []struct {
		name       string
		args       []string
		code       int
		stdout     string
		diagnostic string
	}{
		{"default Markdown", []string{"fetch", server.URL}, 0, "# Reading\n", ""},
		{"format after URL", []string{"fetch", server.URL, "--format", "markdown"}, 0, "# Reading\n", ""},
		{"format before URL", []string{"fetch", "--format=html", server.URL}, 0, "<main><h1>Reading</h1></main><footer>Contact</footer>", ""},
		{"inspectable HTTP error", []string{"fetch", server.URL + "/missing"}, 1, "# Reading\n", "HTTP 404"},
		{"format error", []string{"fetch", server.URL, "--format", "mdx"}, 2, "", "unknown format"},
		{"browser option without browser", []string{"fetch", server.URL, "--wait", "h1"}, 2, "", "requires --js"},
		{"missing URL", []string{"fetch"}, 2, "", "expected one URL"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer

			code := run(t.Context(), tc.args, &stdout, &stderr)
			require.Equal(t, tc.code, code)
			require.Equal(t, tc.stdout, stdout.String())

			if tc.diagnostic == "" {
				require.Empty(t, stderr.String())
			} else {
				require.Contains(t, stderr.String(), tc.diagnostic)
			}
		})
	}
}
