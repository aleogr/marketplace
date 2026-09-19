// Package mailtest holds the expectations every e-mail adapter must meet.
//
// They are written once and run against each adapter, so that a second
// provider is added without re-deriving what an adapter is for
// (docs/roadmap.md, F11). An expectation that only one adapter can meet does
// not belong here: what is written here is the port's contract, not one
// provider's behaviour.
package mailtest

import (
	"testing"

	"github.com/aleogr/marketplace/internal/platform/mail"
)

// Adapter is what the contract is run against: a sender, and a way to ask it
// about an event afterwards.
type Adapter interface {
	mail.Sender
}

// Message is the message the contract sends. It is rendered rather than built
// from a template, because what an adapter receives is always already
// rendered: choosing the words is the platform's job and not the provider's.
func Message(to string) mail.Rendered {
	return mail.Rendered{
		Message: mail.Message{
			Template:    "probe",
			Language:    "en-US",
			To:          to,
			From:        "Marketplace 1",
			Marketplace: "6f1c7b8e-0a6d-4d3a-9a0e-0b2f2a9a1b11",
		},
		Subject: "Marketplace 1 e-mail check",
		Text:    "This message confirms that Marketplace 1 can deliver e-mail.\n",
		HTML:    "<p>This message confirms that Marketplace 1 can deliver e-mail.</p>\n",
	}
}

// Contract runs every expectation of the mail port against one adapter.
func Contract(t *testing.T, build func(t *testing.T) Adapter) {
	t.Helper()

	t.Run("it says which provider it is", func(t *testing.T) {
		adapter := build(t)
		if adapter.Name() == "" {
			// The name is stored beside every identifier the adapter issues,
			// so that a support question has an answer after the provider is
			// replaced (docs/requirements.md, section 25).
			t.Error("the adapter has no name")
		}
	})

	t.Run("it returns the provider's own identifier for a message", func(t *testing.T) {
		adapter := build(t)

		sent, err := adapter.Send(t.Context(), Message("reader@example.test"))
		if err != nil {
			t.Fatalf("Send() = %v, want no error", err)
		}
		if sent.ProviderMessage == "" {
			t.Error("the message was accepted and no identifier came back with it")
		}
	})

	t.Run("it confirms an event about a message it sent", func(t *testing.T) {
		adapter := build(t)

		sent, err := adapter.Send(t.Context(), Message("bouncer@example.test"))
		if err != nil {
			t.Fatalf("Send() = %v, want no error", err)
		}

		confirmed, err := adapter.Confirm(t.Context(), mail.Event{
			Provider: adapter.Name(),
			ID:       "1",
			Message:  sent.ProviderMessage,
			Address:  "bouncer@example.test",
			Kind:     mail.Bounce,
			Reported: "hard_bounce",
		})
		if err != nil {
			t.Fatalf("Confirm() = %v, want no error", err)
		}
		if !confirmed {
			t.Error("the adapter did not confirm an event about a message it had just sent")
		}
	})

	// The case the whole method exists for. A provider that does not sign its
	// webhooks is a provider whose events are claims, and a claim that is
	// acted on without being checked is a stranger deciding which addresses
	// this platform may write to (docs/requirements.md, section 25).
	t.Run("it does not confirm an event nobody reported", func(t *testing.T) {
		adapter := build(t)

		confirmed, err := adapter.Confirm(t.Context(), mail.Event{
			Provider: adapter.Name(),
			ID:       "2",
			Message:  "a-message-that-was-never-sent",
			Address:  "victim@example.test",
			Kind:     mail.Bounce,
			Reported: "hard_bounce",
		})
		if err != nil {
			t.Fatalf("Confirm() = %v, want no error", err)
		}
		if confirmed {
			t.Error("the adapter confirmed an event the provider never reported")
		}
	})

	t.Run("it confirms nothing about an event naming nobody", func(t *testing.T) {
		adapter := build(t)

		confirmed, err := adapter.Confirm(t.Context(), mail.Event{
			Provider: adapter.Name(),
			Kind:     mail.Bounce,
			Reported: "hard_bounce",
		})
		if err != nil {
			t.Fatalf("Confirm() = %v, want no error", err)
		}
		if confirmed {
			t.Error("the adapter confirmed an event that names no address and no message")
		}
	})
}

// Reader runs the expectations of the webhook half of an adapter: the body a
// provider posts becomes the platform's own events, and a body reporting
// something the platform does not act on becomes none.
func Reader(t *testing.T, reader mail.Reader, body []byte, want mail.Event) {
	t.Helper()

	events, err := reader.Events(body)
	if err != nil {
		t.Fatalf("Events() = %v, want no error", err)
	}
	if len(events) != 1 {
		t.Fatalf("Events() returned %d events, want 1: %+v", len(events), events)
	}

	got := events[0]
	switch {
	case got.Provider != reader.Name():
		t.Errorf("the event says it came from %q, want %q", got.Provider, reader.Name())
	case got.Address != want.Address:
		t.Errorf("address = %q, want %q", got.Address, want.Address)
	case got.Kind != want.Kind:
		t.Errorf("kind = %q, want %q", got.Kind, want.Kind)
	case want.Message != "" && got.Message != want.Message:
		t.Errorf("message = %q, want %q", got.Message, want.Message)
	case want.Reported != "" && got.Reported != want.Reported:
		t.Errorf("reported = %q, want %q", got.Reported, want.Reported)
	case got.ID == "":
		// Without it, the same notification arriving twice is two events.
		t.Error("the event carries no identifier of its own")
	}
}
