package mail

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// State is what became of one attempt to send.
type State string

const (
	// StateSent is a message the provider accepted.
	StateSent State = "sent"
	// StateSkipped is a message that was not offered to the provider at all,
	// because the address is suppressed.
	StateSkipped State = "skipped"
	// StateFailed is a message the provider refused or did not answer about.
	StateFailed State = "failed"
)

// Sending is one line of the sending log.
type Sending struct {
	Marketplace     string
	Address         string
	Template        string
	Language        string
	State           State
	ProviderMessage string
	Detail          string
}

// Suppressed reports whether this address has asked, by bouncing or by
// complaining, never to be written to again.
func Suppressed(ctx context.Context, tx pgx.Tx, address string) (bool, error) {
	var found bool
	err := tx.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM email_suppression WHERE address = $1)`,
		Address(address)).Scan(&found)
	if err != nil {
		return false, fmt.Errorf("cannot read the suppression list: %w", err)
	}
	return found, nil
}

// Suppress adds an address to the list, and reports whether it was not there
// already.
//
// The first reason is kept rather than the last: an address that hard-bounced
// and later drew a complaint stopped receiving mail because of the bounce, and
// that is the answer to "why".
func Suppress(ctx context.Context, tx pgx.Tx, event Event) (bool, error) {
	tag, err := tx.Exec(ctx, `
		INSERT INTO email_suppression (address, reason, provider, provider_event, detail)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (address) DO NOTHING
	`, Address(event.Address), string(event.Kind), event.Provider,
		nullable(event.ID), nullable(event.Describe()))
	if err != nil {
		return false, fmt.Errorf("cannot suppress %s: %w", event.Address, err)
	}
	return tag.RowsAffected() == 1, nil
}

// Record writes one line of the sending log.
//
// A skipped message is recorded like a sent one, and that is the point: an
// address that stopped receiving mail would otherwise look exactly like a
// feature that never ran.
func Record(ctx context.Context, tx pgx.Tx, sending Sending) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO email_send
		    (marketplace_id, address, template, language, state, provider_message, detail)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, nullable(sending.Marketplace), Address(sending.Address), sending.Template,
		sending.Language, string(sending.State), nullable(sending.ProviderMessage),
		nullable(sending.Detail))
	if err != nil {
		return fmt.Errorf("cannot record the %s of %s: %w", sending.State, sending.Template, err)
	}
	return nil
}

// nullable turns an empty string into a NULL, so that "no marketplace" and "no
// provider id" are absences in the database rather than empty text that every
// later query would have to remember to exclude.
func nullable(value string) any {
	if value == "" {
		return nil
	}
	return value
}
