package brevo_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aleogr/marketplace/internal/platform/mail"
	"github.com/aleogr/marketplace/internal/platform/mail/brevo"
	"github.com/aleogr/marketplace/internal/platform/mail/mailtest"
)

const key = "a-key-that-is-never-in-this-repository"

// provider is a server that behaves like Brevo's API: it accepts a message
// against the key, names it, and afterwards reports events about the messages
// it named and about no others.
//
// A stub and not the real service, because tests never reach a real provider
// (docs/requirements.md, section 25). What it is for is the adapter's own
// half of the conversation: the header the key travels in, the shape of the
// body, the field the identifier comes back in, and what an answer that is not
// a success does to a caller.
type provider struct {
	mu sync.Mutex
	// sent maps the identifier this server issued to the address it was for.
	sent map[string]string
	// status, when set, is answered to every call instead.
	status int
}

func newProvider() *provider { return &provider{sent: map[string]string{}} }

func (p *provider) start(t *testing.T) string {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(p.handle))
	t.Cleanup(server.Close)
	return server.URL
}

func (p *provider) handle(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("api-key") != key {
		// What the real API answers a call with no key or the wrong one.
		http.Error(w, `{"code":"unauthorized","message":"Key not found"}`, http.StatusUnauthorized)
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.status != 0 {
		http.Error(w, `{"code":"invalid_parameter","message":"sender not recognised"}`, p.status)
		return
	}

	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/v3/smtp/email":
		var body struct {
			To []struct {
				Email string `json:"email"`
			} `json:"to"`
			Subject     string `json:"subject"`
			TextContent string `json:"textContent"`
			HTMLContent string `json:"htmlContent"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || len(body.To) == 0 {
			http.Error(w, `{"code":"invalid_parameter"}`, http.StatusBadRequest)
			return
		}

		id := "<20260919." + body.To[0].Email + "@smtp-relay.mailin.fr>"
		p.sent[id] = body.To[0].Email

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]string{"messageId": id})

	case r.Method == http.MethodGet && r.URL.Path == "/v3/smtp/statistics/events":
		query := r.URL.Query()

		// The provider's own rules about the window, which it enforces and
		// this stub therefore enforces too. Both were learned from the lab,
		// one refusal at a time, because the stub used to ignore the dates the
		// real service validates: a date in the future was refused every
		// single time, and so was one date sent without the other. The
		// confirmation never worked, and no test said so.
		if (query.Get("startDate") == "") != (query.Get("endDate") == "") {
			http.Error(w,
				`{"code":"missing_parameter","message":"Start and end date both required together"}`,
				http.StatusBadRequest)
			return
		}

		for _, name := range []string{"startDate", "endDate"} {
			value := query.Get(name)
			if value == "" {
				continue
			}
			day, err := time.Parse(time.DateOnly, value)
			if err != nil {
				http.Error(w, `{"code":"invalid_parameter","message":"`+name+` is not a date"}`,
					http.StatusBadRequest)
				return
			}
			if day.After(time.Now().UTC()) {
				http.Error(w,
					`{"code":"invalid_parameter","message":"`+name+` should not be greater than current date"}`,
					http.StatusBadRequest)
				return
			}
		}

		events := []map[string]string{}
		if address, ok := p.sent[query.Get("messageId")]; ok && address == query.Get("email") {
			events = append(events, map[string]string{
				"email":     address,
				"event":     "hardBounce",
				"messageId": query.Get("messageId"),
				"reason":    "the mailbox does not exist",
			})
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"events": events})

	default:
		http.NotFound(w, r)
	}
}

// client returns the adapter talking to a stub of the provider.
func client(t *testing.T, stub *provider) *brevo.Client {
	t.Helper()

	adapter, err := brevo.New(brevo.Settings{
		Key:  key,
		From: brevo.Address{Name: "Marketplace platform", Email: "no-reply@marketplace.example"},
		API:  stub.start(t) + "/v3",
	})
	if err != nil {
		t.Fatalf("brevo.New() = %v", err)
	}
	return adapter
}

func TestTheRealAdapterMeetsTheContract(t *testing.T) {
	mailtest.Contract(t, func(t *testing.T) mailtest.Adapter {
		t.Helper()
		return client(t, newProvider())
	})
}

func TestTheAdapterNeedsAKeyAndAnAddress(t *testing.T) {
	for name, settings := range map[string]brevo.Settings{
		"no key":     {From: brevo.Address{Email: "no-reply@marketplace.example"}},
		"no address": {Key: key},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := brevo.New(settings); err == nil {
				t.Error("brevo.New() accepted settings it cannot send with")
			}
		})
	}
}

// A refusal must reach the caller as a refusal: a send that reported success
// for a 400 would be a message nobody receives and nobody misses.
func TestARefusalIsReported(t *testing.T) {
	stub := newProvider()
	adapter := client(t, stub)
	stub.status = http.StatusBadRequest

	_, err := adapter.Send(t.Context(), mailtest.Message("reader@example.test"))
	if err == nil {
		t.Fatal("Send() reported success for a refusal")
	}
	if !strings.Contains(err.Error(), "400") {
		t.Errorf("the error does not say what the provider answered: %v", err)
	}
	if strings.Contains(err.Error(), key) {
		t.Errorf("the error carries the API key: %v", err)
	}
}

// The key travels in the provider's own header. A key in the wrong place is a
// deployment that sends nothing, which is exactly the failure the stub's 401
// stands for here.
func TestTheKeyIsSentWhereTheProviderExpectsIt(t *testing.T) {
	stub := newProvider()
	adapter, err := brevo.New(brevo.Settings{
		Key:  "the-wrong-key",
		From: brevo.Address{Email: "no-reply@marketplace.example"},
		API:  stub.start(t) + "/v3",
	})
	if err != nil {
		t.Fatalf("brevo.New() = %v", err)
	}

	if _, err := adapter.Send(t.Context(), mailtest.Message("reader@example.test")); err == nil {
		t.Error("Send() reported success while authenticating with the wrong key")
	}
}

func TestTheWebhookBecomesPlatformEvents(t *testing.T) {
	adapter := client(t, newProvider())

	// The body Brevo posts for a hard bounce, as its documentation describes
	// it: the event name, the address, its own event id, the identifier the
	// send returned, and the receiving server's reason.
	mailtest.Reader(t, adapter, []byte(`{
		"event": "hard_bounce",
		"email": "Bouncer@Example.Test",
		"id": 1234567,
		"date": "2026-09-19 10:00:00",
		"message-id": "<20260919.1@smtp-relay.mailin.fr>",
		"reason": "550 5.1.1 the mailbox does not exist",
		"tag": "probe"
	}`), mail.Event{
		Address:  "bouncer@example.test",
		Kind:     mail.Bounce,
		Message:  "<20260919.1@smtp-relay.mailin.fr>",
		Reported: "hard_bounce",
	})
}

func TestAComplaintSuppressesTooAndABatchIsRead(t *testing.T) {
	adapter := client(t, newProvider())

	events, err := adapter.Events([]byte(`[
		{"event":"spam","email":"annoyed@example.test","id":1,"message-id":"<a>"},
		{"event":"hard_bounce","email":"gone@example.test","id":2,"message-id":"<b>"}
	]`))
	if err != nil {
		t.Fatalf("Events() = %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("Events() returned %d events, want 2", len(events))
	}
	if events[0].Kind != mail.Complaint {
		t.Errorf("a spam report became %q, want %q", events[0].Kind, mail.Complaint)
	}
}

// A soft bounce is a full mailbox or a server that was busy. Suppressing an
// address for one would lose a reader who did nothing wrong, and the provider
// retries those itself.
func TestWhatSuppressesNobodyIsIgnored(t *testing.T) {
	adapter := client(t, newProvider())

	for _, event := range []string{"soft_bounce", "deferred", "delivered", "opened", "click"} {
		events, err := adapter.Events([]byte(`{"event":"` + event + `","email":"reader@example.test","id":1}`))
		if err != nil {
			t.Fatalf("Events(%s) = %v", event, err)
		}
		if len(events) != 0 {
			t.Errorf("%s became %d events, want none", event, len(events))
		}
	}
}

func TestABodyThatCannotBeReadIsAnError(t *testing.T) {
	adapter := client(t, newProvider())

	for name, body := range map[string]string{
		"empty":      "",
		"not JSON":   "hard_bounce reader@example.test",
		"wrong type": `{"event": 5}`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := adapter.Events([]byte(body)); err == nil {
				t.Error("Events() accepted a body it cannot read")
			}
		})
	}
}

// A refusal and a failure are different work. The provider rejecting the call
// is this deployment being wrong, which the next attempt is rejected for too;
// the provider being unreachable is a moment, which the next attempt may not
// find (internal/platform/mail.ErrRefused).
func TestARefusalIsToldApartFromAFailure(t *testing.T) {
	for name, testCase := range map[string]struct {
		status  int
		refused bool
	}{
		"the call is wrong":           {status: http.StatusBadRequest, refused: true},
		"the key is not accepted":     {status: http.StatusForbidden, refused: true},
		"the provider is in trouble":  {status: http.StatusInternalServerError, refused: false},
		"the provider is overwhelmed": {status: http.StatusBadGateway, refused: false},
	} {
		t.Run(name, func(t *testing.T) {
			stub := newProvider()
			adapter := client(t, stub)
			stub.status = testCase.status

			_, err := adapter.Confirm(t.Context(), mail.Event{
				Provider: brevo.Name, ID: "1", Message: "<a-message>",
				Address: "reader@example.test", Kind: mail.Bounce, Reported: "hard_bounce",
			})
			if err == nil {
				t.Fatalf("Confirm() reported success for a %d", testCase.status)
			}
			if got := errors.Is(err, mail.ErrRefused); got != testCase.refused {
				t.Errorf("a %d refused = %v, want %v: %v", testCase.status, got, testCase.refused, err)
			}
		})
	}
}

// An event this adapter cannot name is not confirmable. Without the mapping,
// the question degenerates into "does this message exist at all", and any
// delivered message answers yes — so a bounce claimed about a message that was
// delivered would confirm.
func TestAnEventTheAdapterCannotNameIsNotConfirmed(t *testing.T) {
	stub := newProvider()
	adapter := client(t, stub)

	sent, err := adapter.Send(t.Context(), mailtest.Message("reader@example.test"))
	if err != nil {
		t.Fatalf("Send() = %v", err)
	}

	confirmed, err := adapter.Confirm(t.Context(), mail.Event{
		Provider: brevo.Name, ID: "1", Message: sent.ProviderMessage,
		Address: "reader@example.test", Kind: mail.Bounce, Reported: "something_new",
	})
	if err != nil {
		t.Fatalf("Confirm() = %v", err)
	}
	if confirmed {
		t.Error("an event the adapter has no name for was confirmed by the mere existence of the message")
	}
}

// The provider spells one event several ways: `hardBounce` where a webhook
// subscribes to it, `hardBounces` where its statistics are queried, and
// `hard_bounce` in the body it posts. Only the first two are in its published
// specification; the third is documentation this platform cannot read from
// here, and a bounce ignored because of an underscore is a bounce nobody
// notices.
func TestEverySpellingOfAnEventIsTheSameEvent(t *testing.T) {
	adapter := client(t, newProvider())

	for _, spelling := range []string{"hard_bounce", "hardBounce", "HARD_BOUNCE", "hard-bounce", "hardBounces"} {
		events, err := adapter.Events([]byte(
			`{"event":"` + spelling + `","email":"gone@example.test","id":1,"message-id":"<a>"}`))
		if err != nil {
			t.Fatalf("Events(%s) = %v", spelling, err)
		}
		if len(events) != 1 {
			t.Errorf("%q became %d events, want 1", spelling, len(events))
			continue
		}
		if events[0].Kind != mail.Bounce {
			t.Errorf("%q became %q, want %q", spelling, events[0].Kind, mail.Bounce)
		}
	}
}

// And the same spelling travels into the question the provider is asked, which
// takes a fourth form again.
func TestAnySpellingIsConfirmedByTheProvidersOwnName(t *testing.T) {
	stub := newProvider()
	adapter := client(t, stub)

	sent, err := adapter.Send(t.Context(), mailtest.Message("gone@example.test"))
	if err != nil {
		t.Fatalf("Send() = %v", err)
	}

	for _, spelling := range []string{"hard_bounce", "hardBounce"} {
		confirmed, err := adapter.Confirm(t.Context(), mail.Event{
			Provider: brevo.Name, ID: "1", Message: sent.ProviderMessage,
			Address: "gone@example.test", Kind: mail.Bounce, Reported: spelling,
		})
		if err != nil {
			t.Fatalf("Confirm(%s) = %v", spelling, err)
		}
		if !confirmed {
			t.Errorf("an event reported as %q was not confirmed", spelling)
		}
	}
}
