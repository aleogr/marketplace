// Package mailbox is the e-mail adapter that sends nothing.
//
// It writes each message as a file in a directory, which is what makes a
// message something a test can read: the end-to-end suite opens the directory
// and finds exactly what a reader would have received, subject and both parts
// (docs/requirements.md, section 25). It is also what an environment whose
// provider account is not ready yet runs with, selected by `PROVIDERS_MODE`.
package mailbox

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/aleogr/marketplace/internal/platform/mail"
)

// Name is what this adapter records itself as beside every identifier it
// issues (docs/requirements.md, section 25).
const Name = "mailbox"

// Message is one file in the directory: everything a reader would have
// received, and what the platform knew about it.
type Message struct {
	ID          string            `json:"id"`
	Written     time.Time         `json:"written"`
	To          string            `json:"to"`
	From        string            `json:"from"`
	Subject     string            `json:"subject"`
	Text        string            `json:"text"`
	HTML        string            `json:"html"`
	Template    string            `json:"template"`
	Language    string            `json:"language"`
	Marketplace string            `json:"marketplace"`
	Variables   map[string]string `json:"variables,omitempty"`
}

// Mailbox is a directory messages are written to.
type Mailbox struct {
	directory string

	// mu guards what this process remembers having sent, which is what lets
	// Confirm answer for events about messages it issued.
	mu   sync.Mutex
	sent map[string]bool
}

// New returns an adapter writing to directory, creating it if it is not there.
func New(directory string) (*Mailbox, error) {
	if directory == "" {
		return nil, fmt.Errorf("the mailbox needs a directory to write to")
	}
	// 0o700: the messages are somebody's mail, and on a shared machine the
	// difference between this and the default is who else can read it.
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return nil, fmt.Errorf("cannot open the mailbox %s: %w", directory, err)
	}
	return &Mailbox{directory: directory, sent: map[string]bool{}}, nil
}

// Name reports which provider this is.
func (m *Mailbox) Name() string { return Name }

// Directory is where the messages are written.
func (m *Mailbox) Directory() string { return m.directory }

// Send writes the message as a file and returns the name of that file as the
// provider's identifier.
func (m *Mailbox) Send(_ context.Context, rendered mail.Rendered) (mail.Sent, error) {
	id, err := identifier()
	if err != nil {
		return mail.Sent{}, err
	}

	message := Message{
		ID:          id,
		Written:     time.Now().UTC(),
		To:          mail.Address(rendered.To),
		From:        rendered.From,
		Subject:     rendered.Subject,
		Text:        rendered.Text,
		HTML:        rendered.HTML,
		Template:    rendered.Template,
		Language:    rendered.Language,
		Marketplace: rendered.Marketplace,
		Variables:   rendered.Variables,
	}

	body, err := json.MarshalIndent(message, "", "  ")
	if err != nil {
		return mail.Sent{}, fmt.Errorf("cannot write the message: %w", err)
	}

	// Written beside the target and renamed into place, so that a reader who
	// lists the directory never opens a file that is still being written.
	final := filepath.Join(m.directory, id+".json")
	temporary := final + ".writing"
	if err := os.WriteFile(temporary, body, 0o600); err != nil {
		return mail.Sent{}, fmt.Errorf("cannot write %s: %w", final, err)
	}
	if err := os.Rename(temporary, final); err != nil {
		return mail.Sent{}, fmt.Errorf("cannot write %s: %w", final, err)
	}

	m.mu.Lock()
	m.sent[id] = true
	m.mu.Unlock()

	return mail.Sent{ProviderMessage: id}, nil
}

// Confirm reports whether this adapter really sent the message an event is
// about.
//
// The fake answers the same question the real adapter does, and answers it the
// same way: from what the provider knows, not from what the caller claims. An
// event about a message this mailbox never wrote is not confirmed.
func (m *Mailbox) Confirm(_ context.Context, event mail.Event) (bool, error) {
	// An event naming no message, or nobody, is not something a provider can
	// be asked about. The real adapter answers the same way
	// (internal/platform/mail/brevo).
	if event.Message == "" || mail.Address(event.Address) == "" {
		return false, nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.sent[event.Message] {
		return true, nil
	}

	// A message this process did not send may still be in the directory: the
	// end-to-end suite restarts the binary, and a deployment has more than one
	// instance.
	_, err := os.Stat(filepath.Join(m.directory, event.Message+".json"))
	return err == nil, nil
}

// identifier returns a name no other message has.
func identifier() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("cannot name the message: %w", err)
	}
	return hex.EncodeToString(raw), nil
}

// Events reads what a caller posted to this adapter's webhook.
//
// The fake has no provider behind it, so it accepts the platform's own event
// shape rather than imitating a provider's. That is what lets a test post a
// bounce and watch the whole path — endpoint, outbox, consumer, suppression —
// run exactly as it runs for the real one (docs/requirements.md, section 25).
func (m *Mailbox) Events(body []byte) ([]mail.Event, error) {
	var posted []mail.Event
	trimmed := bytes.TrimSpace(body)
	switch {
	case len(trimmed) == 0:
		return nil, fmt.Errorf("the mailbox webhook was posted an empty body")
	case trimmed[0] == '[':
		if err := json.Unmarshal(trimmed, &posted); err != nil {
			return nil, fmt.Errorf("the mailbox webhook body cannot be read: %w", err)
		}
	default:
		var one mail.Event
		if err := json.Unmarshal(trimmed, &one); err != nil {
			return nil, fmt.Errorf("the mailbox webhook body cannot be read: %w", err)
		}
		posted = []mail.Event{one}
	}

	events := make([]mail.Event, 0, len(posted))
	for _, event := range posted {
		if mail.Address(event.Address) == "" {
			continue
		}
		if event.Kind != mail.Bounce && event.Kind != mail.Complaint {
			// The same rule the real adapter follows: only what suppresses an
			// address is acted on.
			continue
		}
		event.Provider = Name
		event.Address = mail.Address(event.Address)
		events = append(events, event)
	}
	return events, nil
}
