package scrapemateapp_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gosom/scrapemate"
	"github.com/gosom/scrapemate/mock"
	"github.com/gosom/scrapemate/scrapemateapp"
)

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
