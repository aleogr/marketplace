package mail

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"

	"github.com/aleogr/marketplace/internal/platform/outbox"
)

// The kinds of event this package writes and consumes.
//
// Both go through the outbox rather than being done where they are decided: a
// sale that committed must send its confirmation even if the provider is down
// at that moment, and a webhook must be acknowledged in milliseconds and acted
// on afterwards (docs/requirements.md, section 25).
const (
	// KindSend is a message the platform wants sent.
	KindSend = "email.send"
	// KindEvent is a provider telling the platform what became of one.
	KindEvent = "email.event"
)

// Database is the part of the pool this package uses.
type Database interface {
	InTx(ctx context.Context, fn func(pgx.Tx) error) error
	InTxFor(ctx context.Context, marketplaceID string, fn func(pgx.Tx) error) error
}

// Request asks for a message to be sent, in the transaction that decided it
// should be.
//
// It is the only way the rest of the application sends mail: nothing calls a
// provider from inside a request. An operation that rolls back sends nothing,
// and one that commits always has its message waiting, even if the process
// dies immediately afterwards (docs/design.md, section 2.2).
func Request(ctx context.Context, tx pgx.Tx, message Message) error {
	payload, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("cannot record the %s message: %w", message.Template, err)
	}
	return outbox.Write(ctx, tx, outbox.Event{
		Marketplace: message.Marketplace,
		Kind:        KindSend,
		Queue:       outbox.Notifications,
		Payload:     payload,
	})
}

// Received records a provider's event, in the transaction that answers the
// webhook.
//
// The endpoint acknowledges and this is acted on afterwards, because providers
// give seconds to acknowledge and a provider that times out retries what it
// already delivered (docs/requirements.md, section 25).
func Received(ctx context.Context, tx pgx.Tx, event Event) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("cannot record the %s event: %w", event.Kind, err)
	}
	return outbox.Write(ctx, tx, outbox.Event{
		Kind:    KindEvent,
		Queue:   outbox.Webhooks,
		Payload: payload,
	})
}

// Mailer sends what the outbox hands it, and applies what a provider reports.
type Mailer struct {
	// database is nil in a process running without one. That is a real mode of
	// this binary, not an oversight: the delivery probe sends one message from
	// a process that has no database, and the end-to-end suite runs the whole
	// binary that way (internal/platform/config.Database). Without a database
	// there is no suppression list to consult and no log to write, and the
	// message is sent all the same — the outbox, which is the only other way a
	// message is ever sent, does not exist without one either.
	database  Database
	templates *Templates
	sender    Sender
	// fallback is the language a template missing in the reader's own is
	// rendered in.
	fallback string
	log      *slog.Logger
}

// NewMailer returns the sending half of the mail port.
func NewMailer(database Database, templates *Templates, sender Sender, fallback string, log *slog.Logger) *Mailer {
	return &Mailer{database: database, templates: templates, sender: sender, fallback: fallback, log: log}
}

// Send renders one message and hands it to the provider, unless the address is
// suppressed.
//
// The suppression check is a query and not a cache: an address suppressed a
// second ago must not receive the message this call was about to send, and the
// alternative — a list held in memory by each instance — would be wrong for as
// long as that instance lives.
func (m *Mailer) Send(ctx context.Context, message Message) error {
	message.To = Address(message.To)
	if message.To == "" {
		return errors.New("a message with no address cannot be sent")
	}

	var suppressed bool
	if m.database != nil {
		if err := m.inTx(ctx, message.Marketplace, func(tx pgx.Tx) error {
			var err error
			suppressed, err = Suppressed(ctx, tx, message.To)
			return err
		}); err != nil {
			return err
		}
	}

	if suppressed {
		m.log.InfoContext(ctx, "a message was not sent to a suppressed address",
			"template", message.Template, "provider", m.sender.Name())
		return m.record(ctx, Sending{
			Marketplace: message.Marketplace,
			Address:     message.To,
			Template:    message.Template,
			Language:    message.Language,
			State:       StateSkipped,
			Detail:      "the address is suppressed",
		})
	}

	rendered, err := m.templates.Render(message, m.fallback)
	if err != nil {
		// A template that does not exist will not exist on the next attempt
		// either, so the failure is recorded and reported once rather than
		// being retried until the queue gives up.
		if recordErr := m.record(ctx, Sending{
			Marketplace: message.Marketplace,
			Address:     message.To,
			Template:    message.Template,
			Language:    message.Language,
			State:       StateFailed,
			Detail:      err.Error(),
		}); recordErr != nil {
			return recordErr
		}
		return err
	}

	// The provider is called outside a transaction, on purpose: a network call
	// inside one holds a connection of a very small pool open for as long as
	// somebody else's server takes to answer (internal/platform/db).
	sent, err := m.sender.Send(ctx, rendered)
	if err != nil {
		if recordErr := m.record(ctx, Sending{
			Marketplace: rendered.Marketplace,
			Address:     rendered.To,
			Template:    rendered.Template,
			Language:    rendered.Language,
			State:       StateFailed,
			Detail:      err.Error(),
		}); recordErr != nil {
			return recordErr
		}
		return fmt.Errorf("%s did not accept the message: %w", m.sender.Name(), err)
	}

	return m.record(ctx, Sending{
		Marketplace:     rendered.Marketplace,
		Address:         rendered.To,
		Template:        rendered.Template,
		Language:        rendered.Language,
		State:           StateSent,
		ProviderMessage: sent.ProviderMessage,
	})
}

// Apply acts on an event a provider reported.
//
// It re-reads the event from the provider before believing it. This provider
// does not sign its webhooks, so a call carrying the right shared token is
// still only a claim, and a forged bounce would stop this platform writing to
// an address somebody else chose (docs/requirements.md, section 25).
func (m *Mailer) Apply(ctx context.Context, event Event) error {
	if Address(event.Address) == "" {
		return errors.New("an event about no address cannot be applied")
	}

	confirmed, err := m.sender.Confirm(ctx, event)
	switch {
	case errors.Is(err, ErrRefused):
		// The provider rejected the question itself, which the next attempt
		// would be rejected for too. It is this deployment that is wrong, so
		// it is said once, loudly, and nothing is retried: an event that
		// cannot be checked is an event nobody may act on, and a queue full of
		// the same refusal buries the line that says why.
		m.log.ErrorContext(ctx, "an e-mail event could not be checked with the provider",
			"provider", event.Provider, "event", event.ID, "error", err)
		return nil
	case err != nil:
		return fmt.Errorf("cannot confirm the %s reported by %s: %w", event.Kind, event.Provider, err)
	}
	if !confirmed {
		// Not an error: nothing failed. Somebody claimed something the
		// provider does not report, and the log is where that is visible.
		m.log.WarnContext(ctx, "an e-mail event was not confirmed by the provider and was ignored",
			"provider", event.Provider, "event", event.ID, "kind", string(event.Kind))
		return nil
	}

	if m.database == nil {
		return errors.New("an e-mail event cannot be applied without a database to record it in")
	}

	var added bool
	if err := m.database.InTx(ctx, func(tx pgx.Tx) error {
		var err error
		added, err = Suppress(ctx, tx, event)
		return err
	}); err != nil {
		return err
	}

	if added {
		m.log.InfoContext(ctx, "an address was suppressed",
			"provider", event.Provider, "kind", string(event.Kind), "reason", event.Describe())
	}
	return nil
}

// record writes one line of the sending log, in the marketplace's own
// transaction when it has one.
func (m *Mailer) record(ctx context.Context, sending Sending) error {
	if m.database == nil {
		return nil
	}
	return m.inTx(ctx, sending.Marketplace, func(tx pgx.Tx) error {
		return Record(ctx, tx, sending)
	})
}

// inTx runs fn in a transaction scoped to the marketplace, or in one scoped to
// nothing when the message is the platform's own. Row-level security answers a
// transaction that named no marketplace with the rows that belong to none,
// which is exactly what the platform's own mail is
// (migrations/00007_email.sql).
func (m *Mailer) inTx(ctx context.Context, marketplace string, fn func(pgx.Tx) error) error {
	if marketplace == "" {
		return m.database.InTx(ctx, fn)
	}
	return m.database.InTxFor(ctx, marketplace, fn)
}

// Register makes the mailer the consumer of its two kinds of event.
func Register(registry *outbox.Registry, mailer *Mailer) {
	registry.On(KindSend, outbox.ConsumerFunc{
		Named: "mail.send",
		Do: func(ctx context.Context, event outbox.Event) error {
			var message Message
			if err := json.Unmarshal(event.Payload, &message); err != nil {
				return fmt.Errorf("cannot read the message of event %s: %w", event.ID, err)
			}
			return mailer.Send(ctx, message)
		},
	})

	registry.On(KindEvent, outbox.ConsumerFunc{
		Named: "mail.event",
		Do: func(ctx context.Context, event outbox.Event) error {
			var reported Event
			if err := json.Unmarshal(event.Payload, &reported); err != nil {
				return fmt.Errorf("cannot read the provider event of %s: %w", event.ID, err)
			}
			return mailer.Apply(ctx, reported)
		},
	})
}
