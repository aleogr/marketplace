// Package brevo is the e-mail adapter for Brevo.
//
// It speaks the provider's HTTP API directly rather than through its Go SDK.
// Two endpoints are used — send a message, and re-read an event — and a
// generated SDK for the whole API would be a large dependency, and a large
// attack surface, for two calls (docs/requirements.md, section 25 allows an
// adapter written against the provider's API).
//
// Brevo does not sign its webhooks. That is why Confirm exists: a call
// carrying the right shared token is a claim, and this adapter asks the
// provider whether the claim is true before the platform acts on it.
package brevo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/aleogr/marketplace/internal/platform/mail"
)

// Name is what this adapter records itself as beside every identifier it
// issues, and the last segment of the address its webhooks are posted to.
const Name = "brevo"

// API is the provider's address. It is a field of the client rather than a
// constant here so that the contract tests can run the same adapter against a
// server that behaves like Brevo.
const API = "https://api.brevo.com/v3"

// timeout bounds a call to the provider.
//
// A send happens inside a queued task, so a provider that hangs costs a retry
// rather than a visitor's page; what it must not cost is the instance, which
// would otherwise hold the task open until Cloud Tasks gave up on it.
const timeout = 15 * time.Second

// Client is the adapter.
type Client struct {
	api  string
	key  string
	from Address
	http *http.Client
}

// Address is who the mail comes from at the provider.
//
// The name is the marketplace's and travels with each message; the address is
// the platform's sending domain, which is what DKIM, SPF and DMARC are
// published for (docs/requirements.md, section 25).
type Address struct {
	Name  string
	Email string
}

// Settings is what the adapter needs to exist.
type Settings struct {
	// Key is the API key. It is read from Secret Manager at run time and
	// never appears in this repository.
	Key string
	// From is the fallback sender: the address, and the name used by a message
	// that names no marketplace.
	From Address
	// API overrides the provider's address. Empty is the real one.
	API string
	// HTTP overrides the client, for tests.
	HTTP *http.Client
}

// New returns an adapter talking to Brevo.
func New(settings Settings) (*Client, error) {
	if settings.Key == "" {
		return nil, fmt.Errorf("brevo needs an API key")
	}
	if settings.From.Email == "" {
		return nil, fmt.Errorf("brevo needs an address to send from")
	}

	client := &Client{
		api:  strings.TrimSuffix(cmp(settings.API, API), "/"),
		key:  settings.Key,
		from: settings.From,
		http: settings.HTTP,
	}
	if client.http == nil {
		client.http = &http.Client{Timeout: timeout}
	}
	return client, nil
}

func cmp(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

// Name reports which provider this is.
func (c *Client) Name() string { return Name }

// sendRequest is the body of a send, in the provider's shape.
type sendRequest struct {
	Sender      sender      `json:"sender"`
	To          []recipient `json:"to"`
	Subject     string      `json:"subject"`
	TextContent string      `json:"textContent"`
	HTMLContent string      `json:"htmlContent"`
	// Tags are what a message is found by in the provider's console: the
	// template it came from, and whose it was.
	Tags []string `json:"tags,omitempty"`
}

type sender struct {
	Name  string `json:"name,omitempty"`
	Email string `json:"email"`
}

type recipient struct {
	Email string `json:"email"`
}

// Send hands a message to the provider.
func (c *Client) Send(ctx context.Context, rendered mail.Rendered) (mail.Sent, error) {
	name := rendered.From
	if name == "" {
		name = c.from.Name
	}

	tags := []string{rendered.Template}
	if rendered.Marketplace != "" {
		tags = append(tags, rendered.Marketplace)
	}

	body, err := json.Marshal(sendRequest{
		Sender:      sender{Name: name, Email: c.from.Email},
		To:          []recipient{{Email: mail.Address(rendered.To)}},
		Subject:     rendered.Subject,
		TextContent: rendered.Text,
		HTMLContent: rendered.HTML,
		Tags:        tags,
	})
	if err != nil {
		return mail.Sent{}, fmt.Errorf("cannot build the request: %w", err)
	}

	request, err := c.request(ctx, http.MethodPost, "/smtp/email", bytes.NewReader(body))
	if err != nil {
		return mail.Sent{}, err
	}
	request.Header.Set("Content-Type", "application/json")

	var answer struct {
		MessageID string `json:"messageId"`
	}
	if err := c.do(request, &answer); err != nil {
		return mail.Sent{}, err
	}
	if answer.MessageID == "" {
		return mail.Sent{}, fmt.Errorf("brevo accepted the message and named no identifier for it")
	}
	return mail.Sent{ProviderMessage: answer.MessageID}, nil
}

// queries is what the provider's statistics endpoint calls each of the events
// this platform acts on. The two vocabularies differ — the webhook says
// `hard_bounce` and the query wants `hardBounces` — and this is where that
// ends.
var queries = map[string]string{
	"hard_bounce":   "hardBounces",
	"invalid_email": "invalid",
	"blocked":       "blocked",
	"spam":          "spam",
	"unsubscribed":  "unsubscribed",
}

// Confirm asks the provider whether the event really happened.
//
// It is the re-read section 25 requires of every provider that does not sign
// what it posts: without it, anybody who learned the shared token could stop
// this platform writing to any address they named.
func (c *Client) Confirm(ctx context.Context, event mail.Event) (bool, error) {
	address := mail.Address(event.Address)
	if address == "" {
		return false, nil
	}

	// What the provider calls this event. Without the mapping the question
	// would become "does this message exist at all", which any delivered
	// message answers yes to — and a bounce claimed about a delivered message
	// would confirm. An event this adapter cannot name is not confirmable.
	named, ok := queries[event.Reported]
	if !ok {
		return false, nil
	}

	query := url.Values{}
	query.Set("email", address)
	query.Set("event", named)
	if event.Message != "" {
		query.Set("messageId", event.Message)
	}
	// How far back to look. A webhook that the provider retried for a day is
	// still found, and a window with no beginning would make a stale event
	// look like a current one.
	//
	// There is deliberately no end date. The provider refuses one that is
	// greater than the current date, and "current" is the account's own time
	// zone, which this process does not know: a date computed here in UTC is
	// tomorrow's for an account behind UTC for part of every day. The default
	// end is now, which is what was wanted anyway. A start date in the past is
	// in the past in every time zone.
	query.Set("startDate", time.Now().UTC().AddDate(0, 0, -7).Format(time.DateOnly))

	request, err := c.request(ctx, http.MethodGet, "/smtp/statistics/events?"+query.Encode(), nil)
	if err != nil {
		return false, err
	}

	var answer struct {
		Events []struct {
			Email string `json:"email"`
			Event string `json:"event"`
		} `json:"events"`
	}
	if err := c.do(request, &answer); err != nil {
		return false, err
	}

	for _, reported := range answer.Events {
		if mail.Address(reported.Email) == address {
			return true, nil
		}
	}
	return false, nil
}

// request builds a call to the provider, authenticated.
func (c *Client) request(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	request, err := http.NewRequestWithContext(ctx, method, c.api+path, body)
	if err != nil {
		return nil, fmt.Errorf("cannot build the request to brevo: %w", err)
	}
	request.Header.Set("api-key", c.key)
	request.Header.Set("Accept", "application/json")
	return request, nil
}

// do performs a call and reads its answer.
func (c *Client) do(request *http.Request, into any) error {
	response, err := c.http.Do(request)
	if err != nil {
		return fmt.Errorf("cannot reach brevo: %w", err)
	}
	defer func() { _ = response.Body.Close() }()

	// Bounded, because an answer this adapter cannot understand is an answer
	// it must not read into memory unboundedly either.
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("cannot read brevo's answer: %w", err)
	}

	if response.StatusCode < 200 || response.StatusCode > 299 {
		// The body is the provider's own message, which names the problem
		// ("unrecognised sender", "invalid parameter"). It never carries the
		// key, which is only ever sent.
		err := fmt.Errorf("brevo answered %d: %s", response.StatusCode, summary(body))

		// A refusal is not a failure to try again. The provider understood the
		// call and rejected it, so the next attempt is rejected identically —
		// what is wrong is this deployment, not the moment. Saying so is what
		// keeps one bad call from becoming a retry for every event that
		// follows (internal/platform/mail.Mailer.Apply).
		if response.StatusCode >= 400 && response.StatusCode < 500 {
			return fmt.Errorf("%w: %w", mail.ErrRefused, err)
		}
		return err
	}

	if into == nil {
		return nil
	}
	if err := json.Unmarshal(body, into); err != nil {
		return fmt.Errorf("brevo's answer is not the JSON expected: %w", err)
	}
	return nil
}

// summary shortens a provider's message so that a log line stays a log line.
func summary(body []byte) string {
	const limit = 500
	text := strings.TrimSpace(string(body))
	if len(text) > limit {
		return text[:limit] + "…"
	}
	return text
}
