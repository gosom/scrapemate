package scrapemateapp_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gosom/scrapemate"
	"github.com/gosom/scrapemate/mock"
	"github.com/gosom/scrapemate/scrapemateapp"
)

func TestWithJSKeepsExistingFetcherConfigurationSurface(t *testing.T) {
	t.Parallel()

	writer := &mock.MockResultWriter{}

	cfg, err := scrapemateapp.NewConfig(
		[]scrapemate.ResultWriter{writer},
		scrapemateapp.WithConcurrency(4),
		scrapemateapp.WithBrowserReuseLimit(10),
		scrapemateapp.WithPageReuseLimit(5),
		scrapemateapp.WithJS(scrapemateapp.DisableImages()),
	)
	if err != nil {
		t.Fatalf("NewConfig returned error: %v", err)
	}

	if cfg.Concurrency != 4 || cfg.BrowserReuseLimit != 10 || cfg.PageReuseLimit != 5 || !cfg.UseJS {
		t.Fatalf("expected config surface to remain unchanged: %+v", cfg)
	}
}

func TestDeprecatedRodOptionsAreNoOps(t *testing.T) {
	t.Parallel()

	writer := &mock.MockResultWriter{}

	baseline, err := scrapemateapp.NewConfig(
		[]scrapemate.ResultWriter{writer},
		scrapemateapp.WithJS(),
	)
	require.NoError(t, err)

	compat, err := scrapemateapp.NewConfig(
		[]scrapemate.ResultWriter{writer},
		scrapemateapp.WithJS(
			scrapemateapp.WithBrowserEngine("rod"),
			scrapemateapp.WithRodStealth(),
		),
	)
	require.NoError(t, err)

	require.Equal(t, baseline.UseJS, compat.UseJS)
	require.Equal(t, baseline.JSOpts, compat.JSOpts)
}
