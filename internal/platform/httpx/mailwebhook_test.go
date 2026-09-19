package httpx_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/aleogr/marketplace/internal/platform/httpx"
	"github.com/aleogr/marketplace/internal/platform/i18n"
	"github.com/aleogr/marketplace/internal/platform/mail/mailbox"
	"github.com/aleogr/marketplace/internal/platform/seo"
)

// webhookToken is the shared secret this deployment would have given the
// provider.
const webhookToken = "a-shared-secret"

// written is a transactor that records the statements it was given, and fails
// on demand.
type written struct {
	statements []string
	err        error
}

func (w *written) InTx(_ context.Context, fn func(pgx.Tx) error) error {
	if w.err != nil {
		return w.err
	}
	return fn(&recorder{into: w})
}

type recorder struct {
	pgx.Tx
	into *written
}

func (r *recorder) Exec(_ context.Context, sql string, _ ...any) (pgconn.CommandTag, error) {
	r.into.statements = append(r.into.statements, sql)
	return pgconn.NewCommandTag("INSERT 0 1"), nil
}

// webhook returns the endpoint mounted the way the process mounts it, so that
// the route pattern is part of what is tested and not an assumption.
func webhook(t *testing.T, store *written) (http.Handler, string) {
	t.Helper()

	box, err := mailbox.New(t.TempDir())
	if err != nil {
		t.Fatalf("mailbox.New() = %v", err)
	}
	catalogue, err := i18n.Load()
	if err != nil {
		t.Fatalf("i18n.Load() = %v", err)
	}
	preview, err := seo.NewPreview()
	if err != nil {
		t.Fatalf("seo.NewPreview() = %v", err)
	}

	endpoint := httpx.NewMailWebhook(box, webhookToken, store,
		slog.New(slog.NewTextHandler(io.Discard, nil)))

	return httpx.NewSite(nil, catalogue, preview, false).
		WithMail(endpoint).Handler(), endpoint.Path()
}

// bounce is what the fake provider posts when an address does not exist.
const bounce = `{"ID":"1","Message":"a-message","Address":"gone@example.test",` +
	`"Kind":"bounce","Reported":"hard_bounce"}`

// deliver calls the webhook the way a provider would.
func deliver(t *testing.T, handler http.Handler, path, header, body string) *httptest.ResponseRecorder {
	t.Helper()

	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost,
		"https://marketplace.example"+path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	if header != "" {
		request.Header.Set("X-Webhook-Token", header)
	}

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

// The endpoint acknowledges and writes; acting on the event is the consumer's
// work. Providers allow seconds to acknowledge and re-post what they think
// they failed to deliver (docs/requirements.md, section 25).
func TestAnEventIsAcknowledgedAndWrittenToTheOutbox(t *testing.T) {
	store := &written{}
	handler, path := webhook(t, store)

	response := deliver(t, handler, path, webhookToken, bounce)

	if response.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusAccepted)
	}
	if len(store.statements) != 1 || !strings.Contains(store.statements[0], "outbox_event") {
		t.Errorf("the event was not written to the outbox: %v", store.statements)
	}
}

// The token is the only thing in front of this endpoint, which is why what
// arrives through it is re-read from the provider before it is acted on.
func TestACallWithTheWrongTokenIsRefused(t *testing.T) {
	for name, offered := range map[string]string{
		"none at all":    "",
		"the wrong one":  "not-the-secret",
		"a prefix of it": webhookToken[:5],
	} {
		t.Run(name, func(t *testing.T) {
			store := &written{}
			handler, path := webhook(t, store)

			response := deliver(t, handler, path, offered, bounce)

			if response.Code != http.StatusUnauthorized {
				t.Errorf("status = %d, want %d", response.Code, http.StatusUnauthorized)
			}
			if len(store.statements) != 0 {
				t.Errorf("the event was written anyway: %v", store.statements)
			}
		})
	}
}

// A provider's console may not offer custom headers, so the token is accepted
// in the query string too — which is why it is the second choice: a URL is
// written to every access log it passes through.
func TestTheTokenIsAcceptedInTheQueryString(t *testing.T) {
	store := &written{}
	handler, path := webhook(t, store)

	response := deliver(t, handler, path+"?token="+webhookToken, "", bounce)

	if response.Code != http.StatusAccepted {
		t.Errorf("status = %d, want %d", response.Code, http.StatusAccepted)
	}
}

// Webhooks are received on provider-specific endpoints. The address of a
// provider this deployment does not use leads nowhere.
func TestAnotherProvidersAddressIsNotServed(t *testing.T) {
	store := &written{}
	handler, _ := webhook(t, store)

	response := deliver(t, handler, httpx.MailWebhookPrefix+"someone-else", webhookToken, bounce)

	if response.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", response.Code, http.StatusNotFound)
	}
}

// A body this adapter cannot read will not become readable on a retry.
func TestABodyThatCannotBeReadIsRefused(t *testing.T) {
	store := &written{}
	handler, path := webhook(t, store)

	response := deliver(t, handler, path, webhookToken, "hard_bounce for gone@example.test")

	if response.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}

// An event that reports something this platform does not act on is accepted
// and written nowhere: the provider is answered, and nothing was lost.
func TestWhatSuppressesNobodyIsAcknowledgedAndNotWritten(t *testing.T) {
	store := &written{}
	handler, path := webhook(t, store)

	response := deliver(t, handler, path, webhookToken,
		`{"ID":"1","Address":"reader@example.test","Kind":"delivered"}`)

	if response.Code != http.StatusAccepted {
		t.Errorf("status = %d, want %d", response.Code, http.StatusAccepted)
	}
	if len(store.statements) != 0 {
		t.Errorf("a delivery was written to the outbox: %v", store.statements)
	}
}

// The provider must retry what could not be written down: an event
// acknowledged and lost is an address that keeps receiving mail it bounced.
func TestAnEventThatCannotBeWrittenIsNotAcknowledged(t *testing.T) {
	store := &written{err: errors.New("the database is unreachable")}
	handler, path := webhook(t, store)

	response := deliver(t, handler, path, webhookToken, bounce)

	if response.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", response.Code, http.StatusInternalServerError)
	}
}

// The endpoint is called by a machine that holds no cookie and reads no page,
// so it is outside the CSRF guard — and it must be, or every event would be
// refused with a 403 nobody sees (internal/platform/httpx.Pipeline).
func TestTheWebhookIsOutsideTheCSRFGuard(t *testing.T) {
	store := &written{}
	handler, path := webhook(t, store)

	catalogue, err := i18n.Load()
	if err != nil {
		t.Fatalf("i18n.Load() = %v", err)
	}

	pipeline := httpx.Pipeline{
		Pages:     httpx.NewPages(catalogue),
		Callbacks: []string{path},
		Log:       slog.New(slog.NewTextHandler(io.Discard, nil)),
	}.Wrap(handler)

	response := deliver(t, pipeline, path, webhookToken, bounce)

	if response.Code != http.StatusAccepted {
		t.Errorf("status = %d, want %d: a provider has no token to send", response.Code, http.StatusAccepted)
	}
}
