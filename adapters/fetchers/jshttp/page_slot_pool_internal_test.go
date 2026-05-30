package jshttp

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type fakeSlotFactory struct {
	created atomic.Int64
}

func (f *fakeSlotFactory) newSlot() (*pageSlot, error) {
	f.created.Add(1)

	return &pageSlot{}, nil
}

func TestPageSlotPoolLimitsConcurrentPagesPerBrowser(t *testing.T) {
	factory := &fakeSlotFactory{}
	pool, err := newPageSlotPool(pageSlotPoolConfig{
		poolSize:           1,
		maxPagesPerBrowser: 2,
		factory:            factory,
	})
	require.NoError(t, err)

	defer pool.close()

	ctx := context.Background()
	lease1, err := pool.acquire(ctx)
	require.NoError(t, err)

	defer lease1.release(ctx)

	lease2, err := pool.acquire(ctx)
	require.NoError(t, err)

	defer lease2.release(ctx)

	blocked := make(chan struct{})
	go func() {
		lease3, acquireErr := pool.acquire(ctx)
		require.NoError(t, acquireErr)

		defer lease3.release(ctx)
		close(blocked)
	}()

	require.Never(t, func() bool {
		select {
		case <-blocked:
			return true
		default:
			return false
		}
	}, 100*time.Millisecond, 10*time.Millisecond)

	lease1.release(ctx)

	require.Eventually(t, func() bool {
		select {
		case <-blocked:
			return true
		default:
			return false
		}
	}, time.Second, 10*time.Millisecond)
}

func TestPageSlotPoolCreatesExpectedBrowserCount(t *testing.T) {
	factory := &fakeSlotFactory{}
	pool, err := newPageSlotPool(pageSlotPoolConfig{
		poolSize:           3,
		maxPagesPerBrowser: 4,
		factory:            factory,
	})
	require.NoError(t, err)

	defer pool.close()

	require.Equal(t, int64(3), factory.created.Load())
}
