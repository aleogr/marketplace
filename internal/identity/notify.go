package identity

import (
	"context"
	"maps"

	"github.com/jackc/pgx/v5"

	"github.com/aleogr/marketplace/internal/platform/mail"
)

// notify tells an account's owner by e-mail that something happened to its
// second factors: a method added or removed, a recovery code used (F14
// spec, Flows). It is requested in the transaction that made the change, so a
// change that rolls back tells nobody, and one that commits cannot go
// unannounced.
func notify(ctx context.Context, tx pgx.Tx, v Visit, account Account, template string, variables map[string]string) error {
	all := map[string]string{"Name": account.Name}
	maps.Copy(all, variables)
	return mail.Request(ctx, tx, mail.Message{
		Template: template, Language: v.Language, To: account.Email,
		From: v.MarketplaceName, Marketplace: v.Marketplace, Variables: all,
	})
}
