package brevo

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/aleogr/marketplace/internal/platform/mail"
)

// acted is every event this platform does something about, and what it means
// in the platform's own words.
//
// Everything else the provider reports — delivered, opened, clicked, deferred,
// a soft bounce — is read and ignored on purpose. A soft bounce is a full
// mailbox or a server that was busy, and suppressing an address for it would
// lose a reader who did nothing wrong; the provider retries those itself.
// The keys are normalised, because the provider spells the same event two ways
// and this platform has seen only one of them. Its API names the event
// `hardBounce` when a webhook subscribes to it and `hardBounces` when its
// statistics are queried; the body it posts is documented as `hard_bounce`,
// which is a third spelling and the one nothing here has verified against a
// real bounce. Matching on letters alone costs nothing and makes all three the
// same event — the alternative is a bounce silently ignored because a provider
// changed an underscore.
var acted = map[string]mail.Kind{
	"hardbounce":   mail.Bounce,
	"invalidemail": mail.Bounce,
	"invalid":      mail.Bounce,
	"blocked":      mail.Bounce,
	"spam":         mail.Complaint,
	"unsubscribed": mail.Complaint,
}

// normalise reduces a provider's name for an event to its letters, singular,
// so that `hard_bounce`, `hardBounce`, `HARD-BOUNCE` and `hardBounces` are one
// name. The provider uses the plural in its statistics and the singular in the
// webhook it posts, which is the same event either way.
func normalise(event string) string {
	var letters strings.Builder
	for _, character := range strings.ToLower(strings.TrimSpace(event)) {
		if character >= 'a' && character <= 'z' {
			letters.WriteRune(character)
		}
	}

	// Only a plural of something: `spam`, `blocked`, `invalid` and
	// `unsubscribed` end in no `s`, so nothing this platform acts on loses a
	// letter it needed.
	name := letters.String()
	if len(name) > 1 && strings.HasSuffix(name, "s") {
		name = strings.TrimSuffix(name, "s")
	}
	return name
}

// notification is one event as the provider posts it.
type notification struct {
	Event string `json:"event"`
	Email string `json:"email"`
	// ID is the provider's own identifier for the event. It arrives as a
	// number and is kept as text, because what it is used for is being written
	// down and compared, never counted.
	ID json.Number `json:"id"`
	// MessageID is the identifier the send returned, which is what the event
	// is re-read by.
	MessageID string `json:"message-id"`
	Reason    string `json:"reason"`
}

// Events translates a webhook body into the events the platform acts on.
//
// Brevo posts one event per call, and its batching posts an array of them.
// Both are accepted, because which one arrives is the provider's decision and
// not this platform's.
func (c *Client) Events(body []byte) ([]mail.Event, error) {
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return nil, fmt.Errorf("brevo posted an empty body")
	}

	var posted []notification
	if strings.HasPrefix(trimmed, "[") {
		if err := json.Unmarshal(body, &posted); err != nil {
			return nil, fmt.Errorf("brevo posted a body that cannot be read: %w", err)
		}
	} else {
		var one notification
		if err := json.Unmarshal(body, &one); err != nil {
			return nil, fmt.Errorf("brevo posted a body that cannot be read: %w", err)
		}
		posted = []notification{one}
	}

	var events []mail.Event
	for _, item := range posted {
		kind, ok := acted[normalise(item.Event)]
		if !ok || mail.Address(item.Email) == "" {
			continue
		}
		events = append(events, mail.Event{
			Provider: Name,
			ID:       identifier(item),
			Message:  item.MessageID,
			Address:  mail.Address(item.Email),
			Kind:     kind,
			// What the provider called it, which is what Confirm re-reads by,
			// and its own explanation, which is the answer to "why".
			Reported: item.Event,
			Reason:   strings.TrimSpace(item.Reason),
		})
	}
	return events, nil
}

// identifier is the provider's id for the event, or one built from what it did
// send: a provider that posts no id still posts a message and an event, and
// the pair is what makes the same notification arriving twice recognisable.
func identifier(item notification) string {
	if id := strings.TrimSpace(item.ID.String()); id != "" && id != "0" {
		return id
	}
	if item.MessageID == "" {
		return ""
	}
	return item.MessageID + ":" + item.Event
}
