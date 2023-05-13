package scrapemateapp_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gosom/scrapemate"
	"github.com/gosom/scrapemate/mock"
	"github.com/gosom/scrapemate/scrapemateapp"
)

func Test_NewConfig(t *testing.T) {
	resultwriter := &mock.MockResultWriter{}

	t.Run("default", func(t *testing.T) {
		cfg, err := scrapemateapp.NewConfig([]scrapemate.ResultWriter{resultwriter})
		require.NoError(t, err)
		require.Equal(t, scrapemateapp.DefaultConcurrency, cfg.Concurrency)
	})
	t.Run("with invalid concurrency", func(t *testing.T) {
		_, err := scrapemateapp.NewConfig(
			[]scrapemate.ResultWriter{resultwriter},
			scrapemateapp.WithConcurrency(0),
		)
		require.Error(t, err)
	})
	t.Run("with valid concurrency", func(t *testing.T) {
		cfg, err := scrapemateapp.NewConfig(
			[]scrapemate.ResultWriter{resultwriter},
			scrapemateapp.WithConcurrency(4),
		)
		require.NoError(t, err)
		require.Equal(t, 4, cfg.Concurrency)
	})
	t.Run("with invalid cache type", func(t *testing.T) {
		_, err := scrapemateapp.NewConfig(
			[]scrapemate.ResultWriter{resultwriter},
			scrapemateapp.WithCache("invalid", "path"),
		)
		require.Error(t, err)
	})
	t.Run("with valid cache type", func(t *testing.T) {
		cfg, err := scrapemateapp.NewConfig(
			[]scrapemate.ResultWriter{resultwriter},
			scrapemateapp.WithCache("file", "path"),
		)
		require.NoError(t, err)
		require.Equal(t, "file", cfg.CacheType)
		require.Equal(t, "path", cfg.CachePath)
	})
	t.Run("with js", func(t *testing.T) {
		cfg, err := scrapemateapp.NewConfig(
			[]scrapemate.ResultWriter{resultwriter},
			scrapemateapp.WithJS(),
		)
		require.NoError(t, err)
		require.True(t, cfg.UseJS)
	})
	t.Run("with js and headfull", func(t *testing.T) {
		cfg, err := scrapemateapp.NewConfig(
			[]scrapemate.ResultWriter{resultwriter},
			scrapemateapp.WithJS(scrapemateapp.Headfull()),
		)
		require.NoError(t, err)
		require.True(t, cfg.UseJS)
		require.True(t, cfg.JSOpts.Headfull)
	})
	t.Run("with invalid provider", func(t *testing.T) {
		_, err := scrapemateapp.NewConfig(
			[]scrapemate.ResultWriter{resultwriter},
			scrapemateapp.WithProvider(nil),
		)
		require.Error(t, err)
	})
}
