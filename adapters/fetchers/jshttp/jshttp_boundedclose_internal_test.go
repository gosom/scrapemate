package jshttp

import (
	"errors"
	"testing"
	"time"
)

func TestCloseWithTimeout_FastClose(t *testing.T) {
	done := make(chan struct{})

	go func() {
		closeWithTimeout(func() error { return nil }, time.Second)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("closeWithTimeout did not return for a fast closer")
	}
}

func TestCloseWithTimeout_HungClose(t *testing.T) {
	start := time.Now()
	unblock := make(chan struct{})
	closed := make(chan struct{})

	closeWithTimeout(func() error {
		defer close(closed)

		<-unblock

		return nil
	}, 200*time.Millisecond)

	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("closeWithTimeout did not abandon a hung closer in time: %s", elapsed)
	}

	close(unblock)

	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("closer did not finish after it was unblocked")
	}
}

func TestCloseWithTimeout_ErrorIgnored(_ *testing.T) {
	closeWithTimeout(func() error { return errors.New("boom") }, time.Second)
}
