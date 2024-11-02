package scrapemate_test

import (
	"testing"

	"github.com/gosom/scrapemate"
	"github.com/stretchr/testify/require"
)

func TestNewProxy(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    scrapemate.Proxy
		expectError bool
	}{
		{
			name:  "full socks5 url with credentials",
			input: "socks5://user:pass@example.com:1080",
			expected: scrapemate.Proxy{
				URL:      "socks5://example.com:1080",
				Username: "user",
				Password: "pass",
			},
			expectError: false,
		},
		{
			name:  "http proxy without credentials",
			input: "http://example.com:8080",
			expected: scrapemate.Proxy{
				URL:      "http://example.com:8080",
				Username: "",
				Password: "",
			},
			expectError: false,
		},
		{
			name:  "default to socks5 when no scheme",
			input: "user:pass@example.com:1080",
			expected: scrapemate.Proxy{
				URL:      "socks5://example.com:1080",
				Username: "user",
				Password: "pass",
			},
			expectError: false,
		},
		{
			name:  "only host and port defaults to socks5",
			input: "example.com:1080",
			expected: scrapemate.Proxy{
				URL:      "socks5://example.com:1080",
				Username: "",
				Password: "",
			},
			expectError: false,
		},
		{
			name:  "username only without password",
			input: "socks5://user@example.com:1080",
			expected: scrapemate.Proxy{
				URL:      "socks5://example.com:1080",
				Username: "user",
				Password: "",
			},
			expectError: false,
		},
		{
			name:  "empty password after colon",
			input: "socks5://user:@example.com:1080",
			expected: scrapemate.Proxy{
				URL:      "socks5://example.com:1080",
				Username: "user",
				Password: "",
			},
			expectError: false,
		},
		{
			name:        "invalid scheme",
			input:       "ftp://user:pass@example.com:1080",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proxy, err := scrapemate.NewProxy(tt.input)

			if tt.expectError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.expected.URL, proxy.URL)
			require.Equal(t, tt.expected.Username, proxy.Username)
			require.Equal(t, tt.expected.Password, proxy.Password)
		})
	}
}
