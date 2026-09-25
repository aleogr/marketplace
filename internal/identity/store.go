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
	// Kind is what the account is: buyer or staff (account.kind).
	Kind       UserKind
	Email      string
	Name       string
	VerifiedAt *time.Time
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

const accountColumns = `id::text, marketplace_id::text, kind, email, name, verified_at`

func scanAccount(row pgx.Row) (Account, error) {
	var a Account
	err := row.Scan(&a.ID, &a.MarketplaceID, &a.Kind, &a.Email, &a.Name, &a.VerifiedAt)
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

func accountByID(ctx context.Context, tx pgx.Tx, id string) (Account, error) {
	return scanAccount(tx.QueryRow(ctx, `SELECT `+accountColumns+` FROM account WHERE id = $1`, id))
}

func passwordOf(ctx context.Context, tx pgx.Tx, account string) (string, error) {
	var secret string
	err := tx.QueryRow(ctx,
		`SELECT secret FROM credential WHERE account_id = $1 AND kind = 'password'`, account).Scan(&secret)
	return secret, err
}

// passwordStill locks an account's password and reports whether it is still
// the one a flow verified outside the transaction. A flow that hashed with no
// transaction open writes only if nobody changed the password meanwhile.
func passwordStill(ctx context.Context, tx pgx.Tx, account, verified string) (bool, error) {
	var secret string
	err := tx.QueryRow(ctx,
		`SELECT secret FROM credential WHERE account_id = $1 AND kind = 'password' FOR UPDATE`,
		account).Scan(&secret)
	if err != nil {
		return false, err
	}
	return secret == verified, nil
}

// insertSession opens a session at now, the service's clock, which is the one
// every later check of it reads: the column defaults are the database's.
func insertSession(ctx context.Context, tx pgx.Tx, marketplace, account string, hash []byte, ip, agent string, now time.Time) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO session (account_id, marketplace_id, token_hash, ip, user_agent, created_at, last_seen_at)
		VALUES ($1, $2, $3, $4, $5, $6, $6)`, account, marketplace, hash, ip, agent, now)
	return err
}

type sessionRow struct {
	id, account   string
	created, seen time.Time
	steppedUp     *time.Time
	secondFactor  bool
}

func liveSession(ctx context.Context, tx pgx.Tx, hash []byte) (sessionRow, error) {
	var r sessionRow
	err := tx.QueryRow(ctx, `
		SELECT s.id::text, s.account_id::text, s.created_at, s.last_seen_at, s.stepped_up_at,
		       EXISTS (SELECT 1 FROM second_factor f WHERE f.account_id = s.account_id)
		  FROM session s
		 WHERE s.token_hash = $1 AND s.revoked_at IS NULL`, hash).
		Scan(&r.id, &r.account, &r.created, &r.seen, &r.steppedUp, &r.secondFactor)
	return r, err
}

func touchSession(ctx context.Context, tx pgx.Tx, id string, now time.Time) error {
	_, err := tx.Exec(ctx, `UPDATE session SET last_seen_at = $2 WHERE id = $1`, id, now)
	return err
}

func revokeSession(ctx context.Context, tx pgx.Tx, hash []byte, reason string, now time.Time) (string, error) {
	var account string
	err := tx.QueryRow(ctx, `
		UPDATE session SET revoked_at = $3, revoked_reason = $2
		 WHERE token_hash = $1 AND revoked_at IS NULL RETURNING account_id::text`, hash, reason, now).Scan(&account)
	return account, err
}

// sweepSessions deletes the session rows that can never authenticate again
// (Task 12b): created more than SessionLifetime ago, unseen for more than
// SessionIdle, or revoked more than RevokedKept ago. now is the caller's
// clock, not the database's, like every other check made against a session.
// Row-level security scopes the DELETE to one marketplace.
func sweepSessions(ctx context.Context, tx pgx.Tx, now time.Time) (int64, error) {
	tag, err := tx.Exec(ctx, `
		DELETE FROM session
		 WHERE created_at < $1 OR last_seen_at < $2 OR revoked_at < $3`,
		now.Add(-SessionLifetime), now.Add(-SessionIdle), now.Add(-RevokedKept))
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// revokeSessions ends every live session of an account because its password
// changed, at now, the service's clock.
func revokeSessions(ctx context.Context, tx pgx.Tx, account string, now time.Time) error {
	_, err := tx.Exec(ctx, `
		UPDATE session SET revoked_at = $2, revoked_reason = 'password_changed'
		 WHERE account_id = $1 AND revoked_at IS NULL`, account, now)
	return err
}
