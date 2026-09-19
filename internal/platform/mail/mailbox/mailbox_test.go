package mailbox_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aleogr/marketplace/internal/platform/mail"
	"github.com/aleogr/marketplace/internal/platform/mail/mailbox"
	"github.com/aleogr/marketplace/internal/platform/mail/mailtest"
)

func TestTheFakeMeetsTheContract(t *testing.T) {
	mailtest.Contract(t, func(t *testing.T) mailtest.Adapter {
		t.Helper()

		box, err := mailbox.New(t.TempDir())
		if err != nil {
			t.Fatalf("mailbox.New() = %v", err)
		}
		return box
	})
}

// The point of this adapter: a message is a file a test can read, with
// everything a reader would have received in it (docs/roadmap.md, F11).
func TestTheMessageIsReadableFromTheDirectory(t *testing.T) {
	directory := t.TempDir()
	box, err := mailbox.New(directory)
	if err != nil {
		t.Fatalf("mailbox.New() = %v", err)
	}

	sent, err := box.Send(t.Context(), mailtest.Message("Reader@Example.Test"))
	if err != nil {
		t.Fatalf("Send() = %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(directory, sent.ProviderMessage+".json"))
	if err != nil {
		t.Fatalf("the message was not written where its identifier says: %v", err)
	}

	var written mailbox.Message
	if err := json.Unmarshal(raw, &written); err != nil {
		t.Fatalf("the message is not readable JSON: %v", err)
	}

	switch {
	case written.To != "reader@example.test":
		// Lower-cased on the way in, which is the form the suppression list
		// compares against.
		t.Errorf("to = %q, want the address in the form it is compared in", written.To)
	case written.Subject == "":
		t.Error("the message was written without its subject")
	case !strings.Contains(written.Text, "Marketplace 1"):
		t.Errorf("the text part does not carry the message: %q", written.Text)
	case !strings.Contains(written.HTML, "<p>"):
		t.Errorf("the HTML part is not HTML: %q", written.HTML)
	}
}

// A directory listing must never show a file that is still being written: the
// end-to-end suite reads the directory while the process writes to it.
func TestOnlyFinishedMessagesAreVisible(t *testing.T) {
	directory := t.TempDir()
	box, err := mailbox.New(directory)
	if err != nil {
		t.Fatalf("mailbox.New() = %v", err)
	}

	if _, err := box.Send(t.Context(), mailtest.Message("reader@example.test")); err != nil {
		t.Fatalf("Send() = %v", err)
	}

	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatalf("cannot read the directory: %v", err)
	}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") {
			t.Errorf("the directory holds %q, which is not a finished message", entry.Name())
		}
	}
}

func TestTheWebhookReadsThePlatformsOwnEvents(t *testing.T) {
	box, err := mailbox.New(t.TempDir())
	if err != nil {
		t.Fatalf("mailbox.New() = %v", err)
	}

	mailtest.Reader(t, box, []byte(`{
		"ID": "event-1",
		"Message": "a-message",
		"Address": "Bouncer@Example.Test",
		"Kind": "bounce",
		"Reported": "hard_bounce",
		"Reason": "the mailbox does not exist"
	}`), mail.Event{
		Address:  "bouncer@example.test",
		Kind:     mail.Bounce,
		Message:  "a-message",
		Reported: "hard_bounce",
	})
}

func TestTheWebhookIgnoresWhatSuppressesNobody(t *testing.T) {
	box, err := mailbox.New(t.TempDir())
	if err != nil {
		t.Fatalf("mailbox.New() = %v", err)
	}

	events, err := box.Events([]byte(`[{"Address":"reader@example.test","Kind":"delivered"}]`))
	if err != nil {
		t.Fatalf("Events() = %v, want no error", err)
	}
	if len(events) != 0 {
		t.Errorf("a delivery became %d events, want none", len(events))
	}
}

func TestAMailboxWithNoDirectoryIsRefused(t *testing.T) {
	if _, err := mailbox.New(""); err == nil {
		t.Error("mailbox.New(\"\") returned no error; a mailbox with nowhere to write sends nowhere")
	}
}
