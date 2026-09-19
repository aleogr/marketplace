package mail_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/aleogr/marketplace/internal/platform/i18n"
	"github.com/aleogr/marketplace/internal/platform/mail"
)

// adapter is a provider that accepts everything and remembers it, or fails on
// demand.
type adapter struct {
	sent      []mail.Rendered
	confirms  bool
	failSend  error
	failCheck error
	asked     []mail.Event
}

func (a *adapter) Name() string { return "stub" }

func (a *adapter) Send(_ context.Context, rendered mail.Rendered) (mail.Sent, error) {
	if a.failSend != nil {
		return mail.Sent{}, a.failSend
	}
	a.sent = append(a.sent, rendered)
	return mail.Sent{ProviderMessage: "message-1"}, nil
}

func (a *adapter) Confirm(_ context.Context, event mail.Event) (bool, error) {
	a.asked = append(a.asked, event)
	return a.confirms, a.failCheck
}

// call is one statement the mailer ran, with what it ran it with.
type call struct {
	sql  string
	args []any
}

// transaction answers the two queries this package makes, and records what it
// was asked to write.
//
// A fake and not a database: what the statements do is proved against a real
// PostgreSQL by the integration tests, and what is proved here is the
// decision — who is asked, in which order, and what is written down when the
// answer is no.
type transaction struct {
	pgx.Tx
	suppressed bool
	calls      []call
}

func (t *transaction) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	t.calls = append(t.calls, call{sql: sql, args: args})
	return answer{suppressed: t.suppressed}
}

func (t *transaction) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	t.calls = append(t.calls, call{sql: sql, args: args})
	return pgconn.NewCommandTag("INSERT 0 1"), nil
}

type answer struct{ suppressed bool }

func (a answer) Scan(into ...any) error {
	if len(into) == 1 {
		if target, ok := into[0].(*bool); ok {
			*target = a.suppressed
		}
	}
	return nil
}

// database hands out one transaction and remembers which marketplace it was
// scoped to.
type database struct {
	tx          *transaction
	marketplace string
}

func (d *database) InTx(_ context.Context, fn func(pgx.Tx) error) error { return fn(d.tx) }

func (d *database) InTxFor(_ context.Context, marketplace string, fn func(pgx.Tx) error) error {
	d.marketplace = marketplace
	return fn(d.tx)
}

// mailer returns the mailer under test, with the templates the binary carries.
func mailer(t *testing.T, store *database, sender *adapter) *mail.Mailer {
	t.Helper()
	return mail.NewMailer(store, templates(t), sender,
		i18n.Default, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

// wrote reports whether a statement touching table was run, and with which
// arguments.
func wrote(calls []call, table string) (call, bool) {
	for _, ran := range calls {
		if strings.Contains(ran.sql, table) {
			return ran, true
		}
	}
	return call{}, false
}

func message(to string) mail.Message {
	return mail.Message{
		Template: "probe", Language: "pt-BR", To: to, From: "Marketplace 1",
		Marketplace: "6f1c7b8e-0a6d-4d3a-9a0e-0b2f2a9a1b11",
	}
}

func TestAMessageIsRenderedAndHandedToTheProvider(t *testing.T) {
	store, sender := &database{tx: &transaction{}}, &adapter{}

	if err := mailer(t, store, sender).Send(t.Context(), message("Reader@Example.Test")); err != nil {
		t.Fatalf("Send() = %v", err)
	}

	if len(sender.sent) != 1 {
		t.Fatalf("the provider received %d messages, want 1", len(sender.sent))
	}
	sent := sender.sent[0]
	switch {
	case sent.To != "reader@example.test":
		t.Errorf("to = %q, want the address in the form it is compared in", sent.To)
	case sent.Subject != "Teste de entrega de e-mail — Marketplace 1":
		t.Errorf("subject = %q, want the reader's own language", sent.Subject)
	case sent.HTML == "" || sent.Text == "":
		t.Error("the message was sent with only one part")
	}

	if _, ok := wrote(store.tx.calls, "email_send"); !ok {
		t.Error("the send was not recorded")
	}
	if store.marketplace != message("").Marketplace {
		t.Errorf("the record was written for marketplace %q, want the message's own", store.marketplace)
	}
}

// The delivery's own verification: a suppressed address is skipped, and the
// skip is recorded (docs/roadmap.md, F11).
func TestASuppressedAddressIsSkippedAndRecorded(t *testing.T) {
	store, sender := &database{tx: &transaction{suppressed: true}}, &adapter{}

	if err := mailer(t, store, sender).Send(t.Context(), message("gone@example.test")); err != nil {
		t.Fatalf("Send() = %v, want no error: a suppressed address is not a failure", err)
	}

	if len(sender.sent) != 0 {
		t.Error("the message was handed to the provider although the address is suppressed")
	}

	recorded, ok := wrote(store.tx.calls, "email_send")
	if !ok {
		t.Fatal("nothing was recorded; a skipped message must not look like a feature that never ran")
	}
	if !has(recorded.args, string(mail.StateSkipped)) {
		t.Errorf("the record does not say it was skipped: %v", recorded.args)
	}
}

func TestAMessageWithNoAddressIsRefused(t *testing.T) {
	store, sender := &database{tx: &transaction{}}, &adapter{}

	if err := mailer(t, store, sender).Send(t.Context(), message("  ")); err == nil {
		t.Error("Send() accepted a message with no address")
	}
}

// A provider that refused must leave an error behind: that is what makes the
// queue try again (internal/platform/outbox).
func TestAProviderThatRefusesIsReportedAndRecorded(t *testing.T) {
	store := &database{tx: &transaction{}}
	sender := &adapter{failSend: errors.New("the provider is down")}

	err := mailer(t, store, sender).Send(t.Context(), message("reader@example.test"))
	if err == nil {
		t.Fatal("Send() reported success although the provider refused")
	}

	recorded, ok := wrote(store.tx.calls, "email_send")
	if !ok {
		t.Fatal("the failure was not recorded")
	}
	if !has(recorded.args, string(mail.StateFailed)) {
		t.Errorf("the record does not say it failed: %v", recorded.args)
	}
}

// The reason Confirm exists. Brevo does not sign its webhooks, so an event is
// a claim until the provider says otherwise; acting on a forged one would stop
// this platform writing to an address a stranger named
// (docs/requirements.md, section 25).
func TestAnEventTheProviderDoesNotConfirmSuppressesNobody(t *testing.T) {
	store := &database{tx: &transaction{}}
	sender := &adapter{confirms: false}

	err := mailer(t, store, sender).Apply(t.Context(), mail.Event{
		Provider: "stub", ID: "1", Message: "never-sent",
		Address: "victim@example.test", Kind: mail.Bounce, Reported: "hard_bounce",
	})
	if err != nil {
		t.Fatalf("Apply() = %v, want no error: nothing failed", err)
	}

	if len(sender.asked) != 1 {
		t.Error("the provider was not asked whether the event is real")
	}
	if _, ok := wrote(store.tx.calls, "email_suppression"); ok {
		t.Error("an unconfirmed event suppressed an address")
	}
}

func TestAConfirmedEventSuppressesTheAddress(t *testing.T) {
	store := &database{tx: &transaction{}}
	sender := &adapter{confirms: true}

	err := mailer(t, store, sender).Apply(t.Context(), mail.Event{
		Provider: "stub", ID: "1", Message: "message-1",
		Address: "Gone@Example.Test", Kind: mail.Bounce,
		Reported: "hard_bounce", Reason: "550 no such mailbox",
	})
	if err != nil {
		t.Fatalf("Apply() = %v", err)
	}

	suppressed, ok := wrote(store.tx.calls, "email_suppression")
	if !ok {
		t.Fatal("a confirmed bounce suppressed nobody")
	}
	if !has(suppressed.args, "gone@example.test") {
		t.Errorf("the address was not written in the form it is compared in: %v", suppressed.args)
	}
	if !has(suppressed.args, "hard_bounce: 550 no such mailbox") {
		t.Errorf("the record does not say why: %v", suppressed.args)
	}
}

// A provider that cannot be reached is a retry, not a decision.
func TestAnEventThatCannotBeConfirmedIsReported(t *testing.T) {
	store := &database{tx: &transaction{}}
	sender := &adapter{failCheck: errors.New("the provider is down")}

	err := mailer(t, store, sender).Apply(t.Context(), mail.Event{
		Provider: "stub", ID: "1", Address: "gone@example.test", Kind: mail.Bounce,
	})
	if err == nil {
		t.Error("Apply() reported success although the provider could not be asked")
	}
	if _, ok := wrote(store.tx.calls, "email_suppression"); ok {
		t.Error("an address was suppressed on an answer nobody received")
	}
}

// has reports whether one of the arguments is this text.
func has(args []any, want string) bool {
	for _, arg := range args {
		if text, ok := arg.(string); ok && text == want {
			return true
		}
	}
	return false
}
