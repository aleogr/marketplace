package main

import (
	"context"
	"log/slog"
	"time"
)

// localDispatchEvery is how often a process with fake providers empties its own
// outbox.
const localDispatchEvery = time.Second

// dispatchLocally empties the outbox on a timer, in this process.
//
// Only with fake providers and no Cloud Tasks. A deployment's outbox is
// emptied by Cloud Scheduler calling the dispatch job, and there is no
// scheduler on a laptop or in the end-to-end suite: without this, a message
// requested there would sit in the outbox forever and the suite could never
// read the mail a flow sends. It calls the same dispatcher a deployment's job
// calls, so what it proves is the same path.
func dispatchLocally(ctx context.Context, dispatch func(context.Context) (int, error), every time.Duration, log *slog.Logger) {
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := dispatch(ctx); err != nil && ctx.Err() == nil {
				log.WarnContext(ctx, "local dispatch failed", "error", err)
			}
		}
	}
}
