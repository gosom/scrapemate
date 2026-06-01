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

	closeWithTimeout(func() error {
		select {} // block forever
	}, 200*time.Millisecond)

	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("closeWithTimeout did not abandon a hung closer in time: %s", elapsed)
	}
}

func TestCloseWithTimeout_ErrorIgnored(_ *testing.T) {
	closeWithTimeout(func() error { return errors.New("boom") }, time.Second)
}
