package outbox

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"
)

// Consumer does what an event asks for.
//
// It is named, and the name is recorded with the event it handled: two
// consumers of one event each do their own work once.
type Consumer interface {
	Name() string
	Handle(ctx context.Context, event Event) error
}

// ConsumerFunc adapts a function to a Consumer.
type ConsumerFunc struct {
	Named string
	Do    func(ctx context.Context, event Event) error
}

func (c ConsumerFunc) Name() string { return c.Named }

func (c ConsumerFunc) Handle(ctx context.Context, event Event) error { return c.Do(ctx, event) }

// Registry is which consumers care about which kind of event.
type Registry struct {
	consumers map[string][]Consumer
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry { return &Registry{consumers: map[string][]Consumer{}} }

// On registers a consumer for a kind of event.
func (r *Registry) On(kind string, consumer Consumer) {
	r.consumers[kind] = append(r.consumers[kind], consumer)
}

// For returns the consumers of a kind.
func (r *Registry) For(kind string) []Consumer { return r.consumers[kind] }

// Transactor runs work inside a transaction.
type Transactor interface {
	InTx(ctx context.Context, fn func(pgx.Tx) error) error
}

// ErrUnknownKind is what a delivery of an event nobody consumes returns. It is
// not a failure to retry: retrying it would produce the same nothing until the
// queue gave up.
var ErrUnknownKind = errors.New("no consumer for this kind of event")

// Deliver hands an event to its consumers, once each.
//
// Cloud Tasks delivers at least once: a callback that succeeded but whose
// answer was lost arrives again. Each consumer therefore claims the event
// first, in the same transaction as its own work — a claim that is rolled back
// with the work it guarded, so a consumer that failed is tried again, and one
// that succeeded is not.
func Deliver(ctx context.Context, db Transactor, registry *Registry, event Event, log *slog.Logger) error {
	consumers := registry.For(event.Kind)
	if len(consumers) == 0 {
		return fmt.Errorf("%w: %s", ErrUnknownKind, event.Kind)
	}

	for _, consumer := range consumers {
		err := db.InTx(ctx, func(tx pgx.Tx) error {
			claimed, err := claim(ctx, tx, event.ID, consumer.Name())
			if err != nil {
				return err
			}
			if !claimed {
				log.InfoContext(ctx, "an event was delivered again and skipped",
					"event", event.ID, "kind", event.Kind, "consumer", consumer.Name())
				return nil
			}
			return consumer.Handle(ctx, event)
		})
		if err != nil {
			return fmt.Errorf("%s could not handle %s: %w", consumer.Name(), event.Kind, err)
		}
	}
	return nil
}

// claim records that this consumer is handling this event, and reports whether
// it is the first to do so.
func claim(ctx context.Context, tx pgx.Tx, event, consumer string) (bool, error) {
	tag, err := tx.Exec(ctx, `
		INSERT INTO outbox_delivery (event_id, consumer)
		VALUES ($1, $2)
		ON CONFLICT DO NOTHING
	`, event, consumer)
	if err != nil {
		return false, fmt.Errorf("cannot claim the event: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

// Inline is a queuer that delivers events where they are written, without a
// queue at all.
//
// It is the fake of docs/design.md, section 2.2: the end-to-end suite and the
// integration tests exercise the whole path — the outbox, the registry, the
// idempotency — with no Cloud Tasks and no network.
type Inline struct {
	db       Transactor
	registry *Registry
	log      *slog.Logger
}

// NewInline returns a queuer running consumers there and then.
func NewInline(db Transactor, registry *Registry, log *slog.Logger) *Inline {
	return &Inline{db: db, registry: registry, log: log}
}

// Enqueue delivers the event immediately.
func (i *Inline) Enqueue(ctx context.Context, event Event) error {
	return Deliver(ctx, i.db, i.registry, event, i.log)
}
