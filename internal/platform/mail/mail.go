// Package mail is what the platform needs of an e-mail provider, and nothing
// about any particular one.
//
// The port is defined by the application (docs/requirements.md, section 25): a
// template, a language, the variables that fill it, and who it goes to. An
// adapter translates that into one provider's API. Nothing outside this
// package knows which provider is in use, which is what makes replacing one an
// afternoon rather than a project.
package mail

import (
	"context"
	"fmt"
	"strings"
)

// Message is one e-mail the platform wants sent.
type Message struct {
	// Template names the pair of files that render it.
	Template string
	// Language is the BCP 47 tag it is written in. A marketplace's mail is in
	// the language its reader chose, not in the platform's.
	Language string
	// To is the address, as given; it is lower-cased where it is compared.
	To string
	// From is who the reader sees the message from, which is the marketplace
	// rather than the platform: a buyer of one marketplace has never heard of
	// the others. It is a name and not an address — the address belongs to the
	// sending domain and lives in the adapter — and the templates render it.
	From string
	// Marketplace is the identifier of the marketplace whose message this is,
	// empty for the platform's own. It scopes what is written down about the
	// message, and is never shown to anybody.
	Marketplace string
	// Variables fill the template.
	Variables map[string]string
}

// Sent is what an adapter reports back about a message it accepted.
type Sent struct {
	// ProviderMessage is the provider's own id, which is what a support
	// question is answered with.
	ProviderMessage string
}

// Sender hands a rendered message to a provider.
type Sender interface {
	// Name is the provider, recorded beside every identifier it issues
	// (docs/requirements.md, section 25).
	Name() string
	Send(ctx context.Context, rendered Rendered) (Sent, error)
	// Confirm re-reads an event from the provider and reports whether it is
	// real.
	//
	// It exists because this provider does not sign its webhooks. A call that
	// carries the right secret is still only a claim, and acting on a forged
	// bounce would stop mail to an address somebody chose for us
	// (docs/requirements.md, section 25).
	Confirm(ctx context.Context, event Event) (bool, error)
}

// Reader translates what a provider posts to a webhook into the platform's own
// events.
//
// It is separate from Sender because the two are asked different questions —
// "send this" and "what does this body mean" — and because the endpoint that
// needs it (internal/platform/httpx) has no business knowing how a message is
// sent.
type Reader interface {
	// Name is the provider, which is also the last segment of the address it
	// posts to: webhooks are received on provider-specific endpoints
	// (docs/requirements.md, section 25).
	Name() string
	// Events returns what the body reports, and nothing for a body that
	// reports something this platform does not act on. A body that cannot be
	// read at all is an error.
	Events(body []byte) ([]Event, error)
}

// Kind is what happened to a message, in the platform's own words.
type Kind string

const (
	// Bounce is an address that cannot receive: the domain does not exist, the
	// mailbox does not exist.
	Bounce Kind = "bounce"
	// Complaint is a reader who marked the message as spam.
	Complaint Kind = "complaint"
)

// Event is a provider telling the platform what became of a message.
type Event struct {
	Provider string
	// ID is the provider's own identifier for this event, kept so that the
	// same event arriving twice is handled once.
	ID string
	// Message is the provider's id for the message the event is about.
	Message string
	Address string
	Kind    Kind
	// Reported is what the provider called it — `hard_bounce`, `spam` — kept
	// because providers name these differently and because it is what the
	// event is re-read by.
	Reported string
	// Reason is the provider's own explanation, when it gives one: the words
	// the receiving server answered with. It is what a person needs when they
	// ask why an address stopped receiving.
	Reason string
}

// Describe is what is written down about an event: what the provider called
// it, and why, when it said.
func (e Event) Describe() string {
	switch {
	case e.Reason == "":
		return e.Reported
	case e.Reported == "":
		return e.Reason
	default:
		return e.Reported + ": " + e.Reason
	}
}

// Address is the form an address is stored and compared in.
//
// Lower-cased: the domain is case-insensitive by the standard and no mail
// provider in practice distinguishes the local part either, so a suppression
// list that kept the case would let `Name@example.com` through after
// `name@example.com` bounced.
func Address(address string) string {
	return strings.ToLower(strings.TrimSpace(address))
}

// Rendered is a message with its text filled in, ready for a provider.
type Rendered struct {
	Message
	Subject string
	Text    string
	HTML    string
}

// ErrNoTemplate is returned when a message names a template that does not
// exist in that language.
var ErrNoTemplate = fmt.Errorf("no such template")
