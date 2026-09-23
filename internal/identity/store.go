package identity

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// Account is what the flows need to know about one.
type Account struct {
	ID            string
	MarketplaceID string
	Email         string
	Name          string
	VerifiedAt    *time.Time
}

var errTaken = errors.New("identity: address taken")

func insertAccount(ctx context.Context, tx pgx.Tx, marketplace, email, normalised, name string) (string, error) {
	var id string
	err := tx.QueryRow(ctx, `
		INSERT INTO account (marketplace_id, kind, email, email_normalised, name)
		VALUES ($1, 'buyer', $2, $3, $4)
		ON CONFLICT ON CONSTRAINT account_email_is_unique DO NOTHING
		RETURNING id::text`, marketplace, email, normalised, name).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", errTaken
	}
	return id, err
}

const accountColumns = `id::text, marketplace_id::text, email, name, verified_at`

func scanAccount(row pgx.Row) (Account, error) {
	var a Account
	err := row.Scan(&a.ID, &a.MarketplaceID, &a.Email, &a.Name, &a.VerifiedAt)
	return a, err
}

// accountByEmail finds an account of the marketplace the transaction names;
// row-level security is what scopes it.
func accountByEmail(ctx context.Context, tx pgx.Tx, normalised string) (Account, error) {
	return scanAccount(tx.QueryRow(ctx,
		`SELECT `+accountColumns+` FROM account WHERE email_normalised = $1`, normalised))
}

func setPassword(ctx context.Context, tx pgx.Tx, marketplace, account, secret string) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO credential (account_id, marketplace_id, kind, secret)
		VALUES ($1, $2, 'password', $3)
		ON CONFLICT (account_id, kind) DO UPDATE SET secret = EXCLUDED.secret, updated_at = now()`,
		account, marketplace, secret)
	return err
}

func insertVerification(ctx context.Context, tx pgx.Tx, marketplace, account string, hash []byte, expires time.Time) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO email_verification (token_hash, account_id, marketplace_id, expires_at)
		VALUES ($1, $2, $3, $4)`, hash, account, marketplace, expires)
	return err
}

// consumeVerification spends a token and confirms its account, or reports
// pgx.ErrNoRows for a token that is unknown, spent or expired.
func consumeVerification(ctx context.Context, tx pgx.Tx, hash []byte, now time.Time) (string, error) {
	var account string
	if err := tx.QueryRow(ctx, `
		UPDATE email_verification SET used_at = $2
		 WHERE token_hash = $1 AND used_at IS NULL AND expires_at > $2
		RETURNING account_id::text`, hash, now).Scan(&account); err != nil {
		return "", err
	}
	_, err := tx.Exec(ctx,
		`UPDATE account SET verified_at = coalesce(verified_at, $2) WHERE id = $1`, account, now)
	return account, err
}
