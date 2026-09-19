// Package outbox carries work out of a request and into a queue.
//
// Cloud Run scales to zero, so there is no always-on worker (docs/design.md,
// section 2.5). An operation writes its events in the same transaction as the
// data that caused them, which is what keeps the two from disagreeing: a
// rolled-back sale sends no confirmation, and a committed one always has its
// event waiting, even if the process dies in between. A dispatcher then hands
// each event to Cloud Tasks, which calls back into this same service.
package outbox

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// Queue is a class of work. One per class, because a flood of notifications
// must not delay a payment webhook.
type Queue string

const (
	Webhooks      Queue = "webhooks"
	Notifications Queue = "notifications"
	Jobs          Queue = "jobs"
)

// Event is something that happened, and what has to follow from it.
type Event struct {
	ID string
	// Marketplace is empty for the platform's own work.
	Marketplace string
	Kind        string
	Queue       Queue
	Payload     json.RawMessage
	Attempts    int
}

// Write records an event in the transaction that caused it.
//
// It takes a transaction rather than a pool, and that is the whole design: an
// event written on its own connection would survive a rollback, and an event
// written after the commit would be missing if the process died in between.
func Write(ctx context.Context, tx pgx.Tx, event Event) error {
	payload := event.Payload
	if len(payload) == 0 {
		payload = json.RawMessage(`{}`)
	}

	var marketplace any
	if event.Marketplace != "" {
		marketplace = event.Marketplace
	}

	_, err := tx.Exec(ctx, `
		INSERT INTO outbox_event (marketplace_id, kind, queue, payload)
		VALUES ($1, $2, $3, $4)
	`, marketplace, event.Kind, string(event.Queue), payload)
	if err != nil {
		return fmt.Errorf("cannot write the %s event: %w", event.Kind, err)
	}
	return nil
}
