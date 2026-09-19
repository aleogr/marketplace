package outbox

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// attemptsAllowed is how often an event is offered to the queue before it is
// parked. An event retried forever is an event nobody ever looks at.
const attemptsAllowed = 5

// Database is the part of the connection pool the dispatcher uses.
type Database interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// Queuer hands one event to a queue, or says why it could not.
//
// Cloud Tasks satisfies it in a deployment; the inline one in this package
// satisfies it in tests, running the consumers there and then. Nothing else in
// the platform knows which is mounted, which is what lets a test exercise the
// whole path without a queue (docs/design.md, section 2.2).
type Queuer interface {
	Enqueue(ctx context.Context, event Event) error
}

// Dispatcher moves events from the table to the queue.
type Dispatcher struct {
	db     Database
	queuer Queuer
	log    *slog.Logger
	batch  int
}

// NewDispatcher returns a dispatcher handing events to queuer.
func NewDispatcher(db Database, queuer Queuer, log *slog.Logger) *Dispatcher {
	return &Dispatcher{db: db, queuer: queuer, log: log, batch: 100}
}

// Dispatch hands over every event waiting, and reports how many were handed
// over.
//
// Events are read through a function that runs as the owning role, because the
// dispatcher serves every marketplace at once and row-level security would
// otherwise show it none of them (migrations/00006_outbox_and_jobs.sql).
func (d *Dispatcher) Dispatch(ctx context.Context) (int, error) {
	rows, err := d.db.Query(ctx, `
		SELECT id, marketplace_id, kind, queue, payload, attempts
		FROM outbox_pending($1)
	`, d.batch)
	if err != nil {
		return 0, fmt.Errorf("cannot read the events waiting: %w", err)
	}

	var events []Event
	for rows.Next() {
		var event Event
		var marketplace *string
		var payload []byte
		var queue string

		if err := rows.Scan(&event.ID, &marketplace, &event.Kind, &queue, &payload, &event.Attempts); err != nil {
			rows.Close()
			return 0, fmt.Errorf("cannot read an event: %w", err)
		}
		if marketplace != nil {
			event.Marketplace = *marketplace
		}
		event.Queue = Queue(queue)
		event.Payload = json.RawMessage(payload)
		events = append(events, event)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("cannot read the events waiting: %w", err)
	}

	dispatched := 0
	for _, event := range events {
		if err := d.queuer.Enqueue(ctx, event); err != nil {
			d.failed(ctx, event, err)
			continue
		}
		if _, err := d.db.Exec(ctx, "SELECT outbox_dispatched($1)", event.ID); err != nil {
			// The event reached the queue and the table does not know it. It
			// will be offered again, which the consumer's own idempotency
			// absorbs — that is why the consumer records what it has done
			// before doing it.
			d.log.ErrorContext(ctx, "an event was queued and could not be marked",
				"event", event.ID, "kind", event.Kind, "error", err)
			continue
		}
		dispatched++
	}
	return dispatched, nil
}

// failed records an attempt that did not reach the queue, and says so when the
// event is parked: a parked event is work that will not happen until somebody
// looks at it.
func (d *Dispatcher) failed(ctx context.Context, event Event, cause error) {
	var state string
	if err := d.db.QueryRow(ctx, "SELECT outbox_failed($1, $2, $3)",
		event.ID, cause.Error(), attemptsAllowed).Scan(&state); err != nil {
		d.log.ErrorContext(ctx, "an event failed and could not be recorded as failed",
			"event", event.ID, "error", err)
		return
	}

	level := slog.LevelWarn
	if state == "parked" {
		level = slog.LevelError
	}
	d.log.Log(ctx, level, "an event was not handed to the queue",
		"event", event.ID, "kind", event.Kind, "state", state, "error", cause)
}
