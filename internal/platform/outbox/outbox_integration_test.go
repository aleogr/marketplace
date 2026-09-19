//go:build integration

package outbox_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/aleogr/marketplace/internal/platform/config"
	"github.com/aleogr/marketplace/internal/platform/db"
	"github.com/aleogr/marketplace/internal/platform/dbtest"
	"github.com/aleogr/marketplace/internal/platform/jobs"
	"github.com/aleogr/marketplace/internal/platform/outbox"
)

func TestMain(m *testing.M) {
	os.Exit(dbtest.Run(m))
}

func quiet() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// migrated returns a pool whose database carries the schema.
func migrated(t *testing.T) *db.Pool {
	t.Helper()

	settings := config.Database{URL: dbtest.URL(t)}
	pool, err := db.Open(t.Context(), settings)
	if err != nil {
		t.Fatalf("db.Open() = %v, want a pool", err)
	}
	t.Cleanup(pool.Close)

	if err := db.Migrate(t.Context(), pool, settings, quiet()); err != nil {
		t.Fatalf("db.Migrate() = %v, want nil", err)
	}
	return pool
}

// counting is a queuer that remembers what it was given, and can refuse.
type counting struct {
	mu     sync.Mutex
	taken  []outbox.Event
	refuse error
}

func (c *counting) Enqueue(_ context.Context, event outbox.Event) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.refuse != nil {
		return c.refuse
	}
	c.taken = append(c.taken, event)
	return nil
}

func (c *counting) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.taken)
}

// TestAnEventOfARolledBackTransactionIsNeverDelivered is the whole reason the
// outbox is a table and not a queue call.
//
// The event and the data it announces are written together, so work that did
// not happen announces nothing — and work that did always has its event
// waiting, even if the process died between the commit and the queue.
func TestAnEventOfARolledBackTransactionIsNeverDelivered(t *testing.T) {
	pool := migrated(t)
	queue := &counting{}

	wanted := errors.New("the operation failed after writing its event")
	err := pool.InTx(t.Context(), func(tx pgx.Tx) error {
		if err := outbox.Write(t.Context(), tx, outbox.Event{
			Kind: "rollback.test", Queue: outbox.Webhooks,
		}); err != nil {
			return err
		}
		return wanted
	})
	if !errors.Is(err, wanted) {
		t.Fatalf("InTx() = %v, want the operation's own error", err)
	}

	dispatched, err := outbox.NewDispatcher(pool, queue, quiet()).Dispatch(t.Context())
	if err != nil {
		t.Fatalf("Dispatch() = %v", err)
	}
	if dispatched != 0 || queue.count() != 0 {
		t.Errorf("an event of a rolled-back transaction was handed to the queue")
	}
}

func TestACommittedEventIsDispatchedOnce(t *testing.T) {
	pool := migrated(t)
	queue := &counting{}

	if err := pool.InTx(t.Context(), func(tx pgx.Tx) error {
		return outbox.Write(t.Context(), tx, outbox.Event{Kind: "dispatch.test", Queue: outbox.Webhooks})
	}); err != nil {
		t.Fatalf("InTx() = %v", err)
	}

	dispatcher := outbox.NewDispatcher(pool, queue, quiet())
	if dispatched, err := dispatcher.Dispatch(t.Context()); err != nil || dispatched != 1 {
		t.Fatalf("Dispatch() = %d, %v; want 1, nil", dispatched, err)
	}

	// And not again: an event already handed over is not waiting any more.
	if dispatched, err := dispatcher.Dispatch(t.Context()); err != nil || dispatched != 0 {
		t.Errorf("the second Dispatch() = %d, %v; want 0, nil", dispatched, err)
	}
	if queue.count() != 1 {
		t.Errorf("the queue was given %d events, want 1", queue.count())
	}
}

// TestAnEventThatCannotBeQueuedIsRetriedAndThenParked: an event retried
// forever is an event nobody ever looks at.
func TestAnEventThatCannotBeQueuedIsRetriedAndThenParked(t *testing.T) {
	pool := migrated(t)
	queue := &counting{refuse: errors.New("the queue is unreachable")}

	if err := pool.InTx(t.Context(), func(tx pgx.Tx) error {
		return outbox.Write(t.Context(), tx, outbox.Event{Kind: "parking.test", Queue: outbox.Webhooks})
	}); err != nil {
		t.Fatalf("InTx() = %v", err)
	}

	dispatcher := outbox.NewDispatcher(pool, queue, quiet())
	for range 5 {
		if _, err := dispatcher.Dispatch(t.Context()); err != nil {
			t.Fatalf("Dispatch() = %v", err)
		}
	}

	// By kind: these tests share one database, so a query without one reads
	// whichever event another test happened to leave behind.
	var state string
	var attempts int
	if err := pool.QueryRow(t.Context(),
		"SELECT state, attempts FROM outbox_event WHERE kind = 'parking.test'").Scan(&state, &attempts); err != nil {
		t.Fatalf("cannot read the event: %v", err)
	}
	if state != "parked" {
		t.Errorf("after %d failures the event is %q, want parked", attempts, state)
	}

	// And a parked event is not offered again: it waits for a person.
	if dispatched, _ := dispatcher.Dispatch(t.Context()); dispatched != 0 {
		t.Error("a parked event was offered to the queue again")
	}
}

// TestADuplicateDeliveryIsANoOp covers what Cloud Tasks guarantees: at least
// once. A callback that succeeded but whose answer was lost arrives again, and
// a consumer that charged a card must not charge it twice.
func TestADuplicateDeliveryIsANoOp(t *testing.T) {
	pool := migrated(t)

	var handled int
	registry := outbox.NewRegistry()
	registry.On("order.paid", outbox.ConsumerFunc{
		Named: "receipts",
		Do: func(context.Context, outbox.Event) error {
			handled++
			return nil
		},
	})

	var id string
	if err := pool.InTx(t.Context(), func(tx pgx.Tx) error {
		return tx.QueryRow(t.Context(), `
			INSERT INTO outbox_event (kind, queue) VALUES ('order.paid', 'webhooks')
			RETURNING id::text
		`).Scan(&id)
	}); err != nil {
		t.Fatalf("cannot write the event: %v", err)
	}

	event := outbox.Event{ID: id, Kind: "order.paid", Queue: outbox.Webhooks}
	for range 3 {
		if err := outbox.Deliver(t.Context(), pool, registry, event, quiet()); err != nil {
			t.Fatalf("Deliver() = %v", err)
		}
	}

	if handled != 1 {
		t.Errorf("the consumer ran %d times for one event, want 1", handled)
	}
}

// TestAConsumerThatFailsIsNotRecordedAsDone: the claim is written in the same
// transaction as the work, so a consumer that failed is tried again.
func TestAConsumerThatFailsIsNotRecordedAsDone(t *testing.T) {
	pool := migrated(t)

	attempts := 0
	registry := outbox.NewRegistry()
	registry.On("order.paid", outbox.ConsumerFunc{
		Named: "receipts",
		Do: func(context.Context, outbox.Event) error {
			attempts++
			if attempts == 1 {
				return errors.New("the provider timed out")
			}
			return nil
		},
	})

	var id string
	if err := pool.InTx(t.Context(), func(tx pgx.Tx) error {
		return tx.QueryRow(t.Context(), `
			INSERT INTO outbox_event (kind, queue) VALUES ('order.paid', 'webhooks')
			RETURNING id::text
		`).Scan(&id)
	}); err != nil {
		t.Fatalf("cannot write the event: %v", err)
	}

	event := outbox.Event{ID: id, Kind: "order.paid", Queue: outbox.Webhooks}

	if err := outbox.Deliver(t.Context(), pool, registry, event, quiet()); err == nil {
		t.Fatal("Deliver() = nil for a consumer that failed")
	}
	if err := outbox.Deliver(t.Context(), pool, registry, event, quiet()); err != nil {
		t.Fatalf("the retry = %v, want nil", err)
	}
	if attempts != 2 {
		t.Errorf("the consumer ran %d times, want 2: a failed attempt must be retried", attempts)
	}
}

// TestOnlyOneRunOfAJobProceeds is what the lock is for: Cloud Scheduler can
// fire a job whose previous run is still going.
func TestOnlyOneRunOfAJobProceeds(t *testing.T) {
	pool := migrated(t)

	started := make(chan struct{})
	release := make(chan struct{})

	first := jobs.NewRunner(pool, "instance-a", quiet())
	first.Register(jobs.JobFunc{Named: "reconcile", Do: func(context.Context) error {
		close(started)
		<-release
		return nil
	}})

	var secondRan bool
	second := jobs.NewRunner(pool, "instance-b", quiet())
	second.Register(jobs.JobFunc{Named: "reconcile", Do: func(context.Context) error {
		secondRan = true
		return nil
	}})

	done := make(chan error, 1)
	go func() { done <- first.Run(context.Background(), "reconcile") }()

	<-started
	if err := second.Run(t.Context(), "reconcile"); !errors.Is(err, jobs.ErrBusy) {
		t.Errorf("the second run = %v, want ErrBusy", err)
	}
	if secondRan {
		t.Error("the second run did the work while the first was still doing it")
	}

	close(release)
	if err := <-done; err != nil {
		t.Errorf("the first run = %v, want nil", err)
	}

	// And once it has finished, the job may run again: the lease is released
	// by the run that held it, not waited out.
	if err := second.Run(t.Context(), "reconcile"); err != nil {
		t.Errorf("the run after the first finished = %v, want nil", err)
	}
	if !secondRan {
		t.Error("the job never ran again after the first run finished")
	}
}

func TestAnUnknownJobIsRefused(t *testing.T) {
	pool := migrated(t)

	runner := jobs.NewRunner(pool, "instance-a", quiet())
	if err := runner.Run(t.Context(), "no-such-job"); !errors.Is(err, jobs.ErrUnknownJob) {
		t.Errorf("Run() = %v, want ErrUnknownJob", err)
	}
}
