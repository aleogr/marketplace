package httpx

import (
	"crypto/subtle"
	"io"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5"

	"github.com/aleogr/marketplace/internal/platform/mail"
	"github.com/aleogr/marketplace/internal/platform/outbox"
)

// MailWebhookPrefix is where an e-mail provider posts what became of the
// messages it was given.
//
// The provider's name is the last segment, because webhooks are received on
// provider-specific endpoints: two providers post different bodies, and an
// endpoint that guessed which one it was reading would guess wrong on the day
// a second provider is added (docs/requirements.md, section 25).
const MailWebhookPrefix = "/webhooks/email/"

// mailWebhookLimit bounds the body a provider may post.
const mailWebhookLimit = 1 << 20

// MailWebhook receives a provider's events.
//
// It does three things and no more: it checks the shared token, it writes what
// was posted into the outbox, and it answers. Acting on the event happens
// afterwards, in a consumer, because providers allow seconds to acknowledge —
// ten at some, twenty-two at others — and a provider that times out re-posts
// what it already delivered (docs/requirements.md, section 25).
type MailWebhook struct {
	reader mail.Reader
	// token is the shared secret the provider is configured to send. It is the
	// only thing standing in front of this endpoint, which is why what arrives
	// through it is re-read from the provider before anything is acted on
	// (internal/platform/mail.Mailer.Apply).
	token    string
	database outbox.Transactor
	log      *slog.Logger
}

// NewMailWebhook returns the endpoint for one provider.
func NewMailWebhook(reader mail.Reader, token string, database outbox.Transactor, log *slog.Logger) MailWebhook {
	return MailWebhook{reader: reader, token: token, database: database, log: log}
}

// Path is the address this provider posts to.
func (m MailWebhook) Path() string { return MailWebhookPrefix + m.reader.Name() }

// Handle answers a provider's call.
func (m MailWebhook) Handle(w http.ResponseWriter, r *http.Request) {
	if r.PathValue("provider") != m.reader.Name() {
		// A provider this deployment does not use. Not an error to explain:
		// there is nothing here.
		http.NotFound(w, r)
		return
	}

	if !m.authentic(r) {
		m.log.WarnContext(r.Context(), "a call to the mail webhook carried the wrong token",
			"provider", m.reader.Name())
		http.Error(w, "", http.StatusUnauthorized)
		return
	}

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, mailWebhookLimit))
	if err != nil {
		http.Error(w, "", http.StatusBadRequest)
		return
	}

	events, err := m.reader.Events(body)
	if err != nil {
		// A body this adapter cannot read will not become readable on a retry,
		// so it is refused rather than accepted and dropped silently.
		m.log.ErrorContext(r.Context(), "a mail webhook body could not be read",
			"provider", m.reader.Name(), "error", err)
		http.Error(w, "", http.StatusBadRequest)
		return
	}

	if len(events) > 0 {
		err = m.database.InTx(r.Context(), func(tx pgx.Tx) error {
			for _, event := range events {
				if err := mail.Received(r.Context(), tx, event); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			// The provider must retry: what it reported has not been written
			// down anywhere.
			m.log.ErrorContext(r.Context(), "mail events could not be recorded",
				"provider", m.reader.Name(), "error", err)
			http.Error(w, "", http.StatusInternalServerError)
			return
		}
	}

	// Said on the way out, including when the body reported nothing this
	// platform acts on. An endpoint whose whole job is to be called from
	// outside, and which is silent when it works, cannot be told apart from an
	// endpoint nobody is calling — which is exactly the question asked of it
	// the first time a bounce did not arrive.
	m.log.InfoContext(r.Context(), "mail events received",
		"provider", m.reader.Name(), "events", len(events))

	// Accepted, not done. The events are in the outbox and the consumer is
	// what acts on them.
	w.WriteHeader(http.StatusAccepted)
}

// authentic reports whether the call carries the token this deployment gave
// the provider.
//
// The header is where it belongs. The query string is accepted because a
// provider's console may not offer custom headers, and a token in a URL is
// written to every access log it passes through — which is why it is the
// second choice and not the first.
func (m MailWebhook) authentic(r *http.Request) bool {
	if m.token == "" {
		return false
	}

	offered := r.Header.Get("X-Webhook-Token")
	if offered == "" {
		offered = r.URL.Query().Get("token")
	}
	// Constant time: a comparison that returns as soon as two bytes differ
	// tells a patient caller how much of the token it has guessed.
	return subtle.ConstantTimeCompare([]byte(offered), []byte(m.token)) == 1
}
