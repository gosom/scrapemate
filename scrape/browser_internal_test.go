package scrape

import (
	"context"
	"testing"
	"time"

	"github.com/mxschmitt/playwright-go"
	"github.com/stretchr/testify/require"
)

type latePage struct {
	playwright.Page
	closed chan struct{}
}

func (p *latePage) Close(...playwright.PageCloseOptions) error {
	close(p.closed)

	return nil
}

func TestOpenPageClosesLateTab(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	opening := make(chan struct{}, 1)
	started := make(chan struct{})
	release := make(chan struct{})
	page := &latePage{closed: make(chan struct{})}
	done := make(chan error, 1)

	go func() {
		_, err := openPage(ctx, opening, func() (playwright.Page, error) {
			close(started)
			<-release

			return page, nil
		})
		done <- err
	}()

	<-started
	cancel()

	select {
	case err := <-done:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(time.Second):
		t.Error("tab creation blocked cancellation")
	}

	close(release)

	select {
	case <-page.closed:
	case <-time.After(time.Second):
		t.Fatal("late tab was not closed")
	}
}
