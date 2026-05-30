package scrapemateapp

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gosom/scrapemate"
	"github.com/gosom/scrapemate/mock"
)

func Test_NewConfig(t *testing.T) {
	resultwriter := &mock.MockResultWriter{}

	t.Run("default", func(t *testing.T) {
		cfg, err := NewConfig([]scrapemate.ResultWriter{resultwriter})
		require.NoError(t, err)
		require.Equal(t, DefaultConcurrency, cfg.Concurrency)
		require.Equal(t, 1, cfg.MaxPagesPerBrowser)
	})
	t.Run("with invalid concurrency", func(t *testing.T) {
		_, err := NewConfig(
			[]scrapemate.ResultWriter{resultwriter},
			WithConcurrency(0),
		)
		require.Error(t, err)
	})
	t.Run("with valid concurrency", func(t *testing.T) {
		cfg, err := NewConfig(
			[]scrapemate.ResultWriter{resultwriter},
			WithConcurrency(4),
		)
		require.NoError(t, err)
		require.Equal(t, 4, cfg.Concurrency)
	})
	t.Run("with valid max pages per browser", func(t *testing.T) {
		cfg, err := NewConfig(
			[]scrapemate.ResultWriter{resultwriter},
			WithMaxPagesPerBrowser(4),
		)
		require.NoError(t, err)
		require.Equal(t, 4, cfg.MaxPagesPerBrowser)
	})
	t.Run("with invalid max pages per browser", func(t *testing.T) {
		_, err := NewConfig(
			[]scrapemate.ResultWriter{resultwriter},
			WithMaxPagesPerBrowser(0),
		)
		require.Error(t, err)
	})
	t.Run("with valid browser pool size", func(t *testing.T) {
		cfg, err := NewConfig(
			[]scrapemate.ResultWriter{resultwriter},
			WithBrowserPoolSize(2),
		)
		require.NoError(t, err)
		require.Equal(t, 2, cfg.BrowserPoolSize)
	})
	t.Run("with invalid browser pool size", func(t *testing.T) {
		_, err := NewConfig(
			[]scrapemate.ResultWriter{resultwriter},
			WithBrowserPoolSize(-1),
		)
		require.Error(t, err)
	})
	t.Run("with invalid cache type", func(t *testing.T) {
		_, err := NewConfig(
			[]scrapemate.ResultWriter{resultwriter},
			WithCache("invalid", "path"),
		)
		require.Error(t, err)
	})
	t.Run("with valid cache type", func(t *testing.T) {
		cfg, err := NewConfig(
			[]scrapemate.ResultWriter{resultwriter},
			WithCache("file", "path"),
		)
		require.NoError(t, err)
		require.Equal(t, "file", cfg.CacheType)
		require.Equal(t, "path", cfg.CachePath)
	})
	t.Run("with js", func(t *testing.T) {
		cfg, err := NewConfig(
			[]scrapemate.ResultWriter{resultwriter},
			WithJS(),
		)
		require.NoError(t, err)
		require.True(t, cfg.UseJS)
	})
	t.Run("with js and headfull", func(t *testing.T) {
		cfg, err := NewConfig(
			[]scrapemate.ResultWriter{resultwriter},
			WithJS(Headfull()),
		)
		require.NoError(t, err)
		require.True(t, cfg.UseJS)
		require.True(t, cfg.JSOpts.Headfull)
	})
	t.Run("with invalid provider", func(t *testing.T) {
		_, err := NewConfig(
			[]scrapemate.ResultWriter{resultwriter},
			WithProvider(nil),
		)
		require.Error(t, err)
	})
}

func TestConfig_derivedBrowserPoolSize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cfg  Config
		want int
	}{
		{
			name: "explicit browser pool size",
			cfg: Config{
				Concurrency:        8,
				BrowserPoolSize:    5,
				MaxPagesPerBrowser: 3,
			},
			want: 5,
		},
		{
			name: "derived exact division",
			cfg: Config{
				Concurrency:        8,
				MaxPagesPerBrowser: 4,
			},
			want: 2,
		},
		{
			name: "derived ceiling division",
			cfg: Config{
				Concurrency:        9,
				MaxPagesPerBrowser: 4,
			},
			want: 3,
		},
		{
			name: "invalid max pages per browser falls back to one",
			cfg: Config{
				Concurrency:        3,
				MaxPagesPerBrowser: 0,
			},
			want: 3,
		},
		{
			name: "negative max pages per browser falls back to one",
			cfg: Config{
				Concurrency:        4,
				MaxPagesPerBrowser: -2,
			},
			want: 4,
		},
	}

	for _, tc := range tests {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, tc.want, tc.cfg.derivedBrowserPoolSize())
		})
	}
}
