package scrapemate_test

import (
	"testing"

	"github.com/gosom/scrapemate"
	"github.com/gosom/scrapemate/mock"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	t.Parallel()
	provider := &mock.MockProvider{}
	mate, err := scrapemate.New(
		scrapemate.WithJobProvider(provider),
	)
	require.NoError(t, err)
	require.NotNil(t, mate)
}

func TestNewNoProvider(t *testing.T) {
	t.Parallel()
	_, err := scrapemate.New()
	require.Error(t, err)
}
