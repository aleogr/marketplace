//go:build integration

package mail_test

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/aleogr/marketplace/internal/platform/config"
	"github.com/aleogr/marketplace/internal/platform/db"
	"github.com/aleogr/marketplace/internal/platform/dbtest"
	"github.com/aleogr/marketplace/internal/platform/httpx"
	"github.com/aleogr/marketplace/internal/platform/i18n"
	"github.com/aleogr/marketplace/internal/platform/mail"
	"github.com/aleogr/marketplace/internal/platform/mail/mailbox"
	"github.com/aleogr/marketplace/internal/platform/outbox"
	"github.com/aleogr/marketplace/internal/platform/seo"
	"github.com/aleogr/marketplace/internal/tenancy"
)

// token is the shared secret this deployment would have given the provider.
const token = "a-shared-secret"

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
		t.Fatalf("db.Open() = %v", err)
	}
	t.Cleanup(pool.Close)

	if err := db.Migrate(t.Context(), pool, settings, quiet()); err != nil {
		t.Fatalf("db.Migrate() = %v", err)
	}
	return pool
}

// platform is everything the process assembles around the mail port, with the
// fake adapter behind it: the endpoint a provider posts to, the outbox the
// event is written into, the dispatcher that moves it, and the consumer that
// acts on it.
//
// The whole path and not a piece of it, because what this delivery promises is
// that a bounce arriving at the endpoint stops the next message — and every
// step in between is where that can fail (docs/roadmap.md, F11).
type platform struct {
	pool       *db.Pool
	box        *mailbox.Mailbox
	mailer     *mail.Mailer
	dispatcher *outbox.Dispatcher
	webhook    http.Handler
	path       string
}

func assemble(t *testing.T) platform {
	t.Helper()

	pool := migrated(t)

	box, err := mailbox.New(t.TempDir())
	if err != nil {
		t.Fatalf("mailbox.New() = %v", err)
	}

	templates, err := mail.LoadTemplates()
	if err != nil {
		t.Fatalf("mail.LoadTemplates() = %v", err)
	}

	mailer := mail.NewMailer(pool, templates, box, i18n.Default, quiet())

	registry := outbox.NewRegistry()
	mail.Register(registry, mailer)

	catalogue, err := i18n.Load()
	if err != nil {
		t.Fatalf("i18n.Load() = %v", err)
	}
	preview, err := seo.NewPreview()
	if err != nil {
		t.Fatalf("seo.NewPreview() = %v", err)
	}

	endpoint := httpx.NewMailWebhook(box, token, pool, quiet())

	return platform{
		pool:       pool,
		box:        box,
		mailer:     mailer,
		dispatcher: outbox.NewDispatcher(pool, outbox.NewInline(pool, registry, quiet()), quiet()),
		webhook: httpx.NewSite(nil, catalogue, preview, false).
			WithMail(endpoint).Handler(),
		path: endpoint.Path(),
	}
}

// report posts an event the way the provider does, and returns the status.
func (p platform) report(t *testing.T, event mail.Event) int {
	t.Helper()

	body, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("cannot build the event: %v", err)
	}

	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost,
		"https://marketplace.example"+p.path, strings.NewReader(string(body)))
	request.Header.Set("X-Webhook-Token", token)
	request.Header.Set("Content-Type", "application/json")

	response := httptest.NewRecorder()
	p.webhook.ServeHTTP(response, request)
	return response.Code
}

// dispatch moves what is waiting in the outbox to its consumers.
func (p platform) dispatch(t *testing.T) {
	t.Helper()

	if _, err := p.dispatcher.Dispatch(t.Context()); err != nil {
		t.Fatalf("Dispatch() = %v", err)
	}
}

func (p platform) message(to string) mail.Message {
	return mail.Message{Template: "probe", Language: "pt-BR", To: to, From: "Marketplace 1"}
}

// delivered counts the messages the fake adapter wrote.
func (p platform) delivered(t *testing.T) int {
	t.Helper()

	entries, err := os.ReadDir(p.box.Directory())
	if err != nil {
		t.Fatalf("cannot read the mailbox: %v", err)
	}
	return len(entries)
}

// states returns what was recorded about every attempt to write to address.
func (p platform) states(t *testing.T, address string) []string {
	t.Helper()

	var states []string
	err := p.pool.InTx(t.Context(), func(tx pgx.Tx) error {
		rows, err := tx.Query(t.Context(),
			`SELECT state FROM email_send WHERE address = $1 ORDER BY created_at`, address)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var state string
			if err := rows.Scan(&state); err != nil {
				return err
			}
			states = append(states, state)
		}
		return rows.Err()
	})
	if err != nil {
		t.Fatalf("cannot read the sending log: %v", err)
	}
	return states
}

func (p platform) suppressed(t *testing.T, address string) bool {
	t.Helper()

	var found bool
	if err := p.pool.InTx(t.Context(), func(tx pgx.Tx) error {
		var err error
		found, err = mail.Suppressed(t.Context(), tx, address)
		return err
	}); err != nil {
		t.Fatalf("cannot read the suppression list: %v", err)
	}
	return found
}

// TestABounceStopsTheNextMessage is the delivery's own verification: a bounce
// webhook suppresses the address, and a later send to it is skipped and
// recorded as skipped (docs/roadmap.md, F11).
func TestABounceStopsTheNextMessage(t *testing.T) {
	site := assemble(t)
	const address = "gone@example.test"

	if err := site.mailer.Send(t.Context(), site.message(address)); err != nil {
		t.Fatalf("the first message could not be sent: %v", err)
	}
	if site.delivered(t) != 1 {
		t.Fatalf("the provider received %d messages, want 1", site.delivered(t))
	}

	// What the provider reports about it, through the endpoint it posts to.
	sent := site.sentMessage(t, address)
	if status := site.report(t, mail.Event{
		ID: "event-1", Message: sent, Address: "Gone@Example.Test",
		Kind: mail.Bounce, Reported: "hard_bounce", Reason: "550 no such mailbox",
	}); status != http.StatusAccepted {
		t.Fatalf("the webhook answered %d, want %d", status, http.StatusAccepted)
	}

	// Nothing has happened yet: the endpoint acknowledged and wrote, which is
	// all it does.
	if site.suppressed(t, address) {
		t.Fatal("the endpoint suppressed the address itself instead of writing an event")
	}

	site.dispatch(t)

	if !site.suppressed(t, address) {
		t.Fatal("the bounce was delivered and suppressed nobody")
	}

	if err := site.mailer.Send(t.Context(), site.message(address)); err != nil {
		t.Fatalf("the second message returned %v; a suppressed address is not a failure", err)
	}
	if got := site.delivered(t); got != 1 {
		t.Errorf("the provider received %d messages, want the first one only", got)
	}

	states := site.states(t, address)
	want := []string{string(mail.StateSent), string(mail.StateSkipped)}
	if len(states) != len(want) || states[0] != want[0] || states[1] != want[1] {
		t.Errorf("the sending log reads %v, want %v", states, want)
	}
}

// sentMessage returns the provider's identifier for the message sent to
// address, read from the log the platform keeps.
func (p platform) sentMessage(t *testing.T, address string) string {
	t.Helper()

	var id string
	if err := p.pool.InTx(t.Context(), func(tx pgx.Tx) error {
		return tx.QueryRow(t.Context(),
			`SELECT provider_message FROM email_send
			  WHERE address = $1 AND state = 'sent'
			  ORDER BY created_at DESC LIMIT 1`, address).Scan(&id)
	}); err != nil {
		t.Fatalf("cannot read what the provider called the message: %v", err)
	}
	return id
}

// The reason the re-read exists. This provider does not sign what it posts, so
// anybody who learned the shared token could otherwise stop this platform
// writing to any address they named (docs/requirements.md, section 25).
func TestAnEventTheProviderNeverReportedSuppressesNobody(t *testing.T) {
	site := assemble(t)
	const address = "victim@example.test"

	if status := site.report(t, mail.Event{
		ID: "event-2", Message: "a-message-this-platform-never-sent",
		Address: address, Kind: mail.Bounce, Reported: "hard_bounce",
	}); status != http.StatusAccepted {
		t.Fatalf("the webhook answered %d, want %d", status, http.StatusAccepted)
	}

	site.dispatch(t)

	if site.suppressed(t, address) {
		t.Error("an address was suppressed by an event the provider does not report")
	}

	if err := site.mailer.Send(t.Context(), site.message(address)); err != nil {
		t.Fatalf("Send() = %v", err)
	}
	if site.delivered(t) != 1 {
		t.Error("the message was not sent, so the forged event stopped it after all")
	}
}

// The sending log is a marketplace's own. Row-level security answers another
// marketplace's transaction with no rows at all, whether or not the query
// remembered to say so (docs/design.md, section 2.6).
func TestTheSendingLogBelongsToItsMarketplace(t *testing.T) {
	site := assemble(t)

	dbtest.AsApplication(t, site.pool)

	mine := site.marketplace(t, "marketplace-one")
	theirs := site.marketplace(t, "marketplace-two")

	// An address of this test's own: the tests of a package share one
	// database, and a count with no condition would be counting everybody's
	// rows.
	const address = "tenant-reader@example.test"

	message := site.message(address)
	message.Marketplace = mine
	if err := site.mailer.Send(t.Context(), message); err != nil {
		t.Fatalf("Send() = %v", err)
	}

	if got := site.countFor(t, mine, address); got != 1 {
		t.Errorf("the marketplace sees %d of its own sends, want 1", got)
	}
	if got := site.countFor(t, theirs, address); got != 0 {
		t.Errorf("another marketplace sees %d of them, want none", got)
	}
}

// marketplace seeds one and returns its identifier.
func (p platform) marketplace(t *testing.T, slug string) string {
	t.Helper()

	if err := p.pool.InTx(t.Context(), func(tx pgx.Tx) error {
		return tenancy.Seed(t.Context(), tx, []tenancy.Spec{{
			Slug: slug, Name: slug, Market: "BR", RevenueModel: "commission",
			DefaultLanguage: "pt-BR", Languages: []string{"en-US", "pt-BR"},
			Hosts: []string{slug + ".example.test"},
		}})
	}); err != nil {
		t.Fatalf("cannot seed %s: %v", slug, err)
	}

	var id string
	if err := p.pool.InTx(t.Context(), func(tx pgx.Tx) error {
		return tx.QueryRow(t.Context(),
			`SELECT id::text FROM marketplace WHERE slug = $1`, slug).Scan(&id)
	}); err != nil {
		t.Fatalf("cannot read %s: %v", slug, err)
	}
	return id
}

// countFor is how many sends to this address a marketplace's own request can
// see.
//
// As the application role, because row-level security applies to neither a
// superuser nor a table's owner, and the tests connect as both
// (internal/platform/dbtest.AsApplication).
func (p platform) countFor(t *testing.T, marketplace, address string) int {
	t.Helper()

	var count int
	if err := dbtest.Serving(t, p.pool, marketplace, func(tx pgx.Tx) error {
		return tx.QueryRow(t.Context(),
			`SELECT count(*) FROM email_send WHERE address = $1`, address).Scan(&count)
	}); err != nil {
		t.Fatalf("cannot count the sends: %v", err)
	}
	return count
}
