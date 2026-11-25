package scrape

import (
	"context"
	"errors"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type resetTestSession struct {
	session

	discardSession func() error
}

func (s *resetTestSession) close() error   { return nil }
func (s *resetTestSession) discard() error { return s.discardSession() }

func TestResetSessionDiscardsLateReplacement(t *testing.T) {
	t.Parallel()

	client, err := New()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, client.Close()) })

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	started := make(chan struct{})
	discarded := make(chan struct{})
	original := client.session
	client.newSession = func(context.Context, *url.URL) (session, error) {
		close(started)
		<-ctx.Done()

		return &resetTestSession{discardSession: func() error {
			close(discarded)

			return nil
		}}, nil
	}

	resetCtx, cancelReset := context.WithCancel(ctx)
	defer cancelReset()

	reset := make(chan error, 1)
	go func() { reset <- client.ResetSession(resetCtx) }()

	select {
	case <-started:
	case <-ctx.Done():
		t.Fatal("replacement never started")
	}

	cancelReset()

	select {
	case resetErr := <-reset:
		require.ErrorIs(t, resetErr, context.Canceled)
	case <-ctx.Done():
		t.Fatal("reset blocked on replacement startup")
	}

	require.Same(t, original, client.session)
	require.NoError(t, client.Close())
	cancel()

	select {
	case <-discarded:
	case <-time.After(5 * time.Second):
		t.Fatal("the abandoned replacement was not discarded")
	}
}

func TestResetSessionCancellationDuringCleanup(t *testing.T) {
	t.Parallel()

	client, err := New()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, client.Close()) })

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	started := make(chan struct{})
	discarded := make(chan struct{})
	fresh := client.session
	client.session = &resetTestSession{discardSession: func() error {
		close(started)
		<-ctx.Done()
		close(discarded)

		return nil
	}}
	client.newSession = func(context.Context, *url.URL) (session, error) {
		return fresh, nil
	}

	resetCtx, cancelReset := context.WithCancel(ctx)
	defer cancelReset()

	reset := make(chan error, 1)
	go func() { reset <- client.ResetSession(resetCtx) }()

	select {
	case <-started:
	case <-ctx.Done():
		t.Fatal("cleanup never started")
	}

	cancelReset()

	select {
	case resetErr := <-reset:
		require.ErrorIs(t, resetErr, context.Canceled)
	case <-ctx.Done():
		t.Fatal("reset blocked on cleanup")
	}

	require.Same(t, fresh, client.session)
	require.NoError(t, client.Close())
	cancel()

	select {
	case <-discarded:
	case <-time.After(5 * time.Second):
		t.Fatal("cleanup did not finish")
	}
}

func TestResetSessionBackendFailures(t *testing.T) {
	t.Parallel()

	backendErr := errors.New("backend failure")
	for _, tc := range []struct {
		name       string
		startErr   error
		discardErr error
	}{
		{name: "startup", startErr: backendErr},
		{name: "cleanup", discardErr: backendErr},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client, err := New()
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, client.Close()) })

			require.NoError(t, client.session.close())

			discarded := false
			original := &resetTestSession{discardSession: func() error {
				discarded = true

				return tc.discardErr
			}}
			fresh := &resetTestSession{}
			client.session = original
			client.newSession = func(context.Context, *url.URL) (session, error) {
				return fresh, tc.startErr
			}

			require.ErrorIs(t, client.ResetSession(t.Context()), backendErr)

			if tc.startErr != nil {
				require.Same(t, original, client.session)
				require.False(t, discarded)

				return
			}

			require.Same(t, fresh, client.session)
			require.True(t, discarded)
		})
	}
}
