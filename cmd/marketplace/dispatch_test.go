package main

import (
	"context"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"
)

func TestDispatchLocallyRunsUntilItsContextEnds(t *testing.T) {
	var calls atomic.Int32
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		dispatchLocally(ctx, func(context.Context) (int, error) {
			if calls.Add(1) == 3 {
				cancel()
			}
			return 0, nil
		}, time.Millisecond, slog.New(slog.NewTextHandler(io.Discard, nil)))
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("dispatchLocally did not return after its context was cancelled")
	}
	if got := calls.Load(); got < 3 {
		t.Fatalf("dispatch ran %d times, want at least 3", got)
	}
}
