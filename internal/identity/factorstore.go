package identity

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// EnrolmentLifetime is how long an app or a key being added waits for the
// answer that proves it works: long enough to find the phone and install an
// app, short enough that a page left open is not a way in later.
const EnrolmentLifetime = 15 * time.Minute

// factor is a second_factor row.
type factor struct {
	ID           string
	Method       Method
	Label        string
	Secret       []byte
	LastStep     *int64
	CredentialID []byte
	PublicKey    []byte
	SignCount    int64
	Flags        int16
	CreatedAt    time.Time
	LastUsedAt   *time.Time
}

const factorColumns = `id::text, kind, label, secret, totp_last_step, credential_id, public_key,
	coalesce(sign_count, 0), coalesce(credential_flags, 0), created_at, last_used_at`

func scanFactor(row pgx.Row) (factor, error) {
	var f factor
	err := row.Scan(&f.ID, &f.Method, &f.Label, &f.Secret, &f.LastStep, &f.CredentialID, &f.PublicKey,
		&f.SignCount, &f.Flags, &f.CreatedAt, &f.LastUsedAt)
	return f, err
}

// factorsOf returns an account's second factors, oldest first.
func factorsOf(ctx context.Context, tx pgx.Tx, account string) ([]factor, error) {
	rows, err := tx.Query(ctx,
		`SELECT `+factorColumns+` FROM second_factor WHERE account_id = $1 ORDER BY created_at, id`, account)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (factor, error) { return scanFactor(row) })
}

func insertApp(ctx context.Context, tx pgx.Tx, marketplace, account, label string, sealed []byte, step int64, now time.Time) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO second_factor (account_id, marketplace_id, kind, label, secret, totp_last_step, created_at)
		VALUES ($1, $2, 'totp', $3, $4, $5, $6)`, account, marketplace, label, sealed, step, now)
	return err
}

// lockAccount takes an account's row lock so that concurrent requests that
// touch its factors and recovery codes serialise instead of each reading a
// state the other is about to change. FOR NO KEY UPDATE, not FOR UPDATE, so
// it does not block the FOR KEY SHARE lock a session insert takes on the same
// row at sign-in. Exec, not Scan: a staff account's row is invisible to the
// application role under RLS, and a missing row must not be an error.
func lockAccount(ctx context.Context, tx pgx.Tx, account string) error {
	_, err := tx.Exec(ctx, `SELECT 1 FROM account WHERE id = $1 FOR NO KEY UPDATE`, account)
	return err
}

// deleteFactor removes one of an account's factors and returns what it was,
// or pgx.ErrNoRows for one the account does not have. An id that is not a
// well-formed UUID is that too, rather than a database error: id's column is
// a uuid, and PostgreSQL refuses to compare it against text that is not one.
func deleteFactor(ctx context.Context, tx pgx.Tx, account, id string) (factor, error) {
	var parsed pgtype.UUID
	if err := parsed.Scan(id); err != nil {
		return factor{}, pgx.ErrNoRows
	}
	if err := lockAccount(ctx, tx, account); err != nil {
		return factor{}, err
	}
	return scanFactor(tx.QueryRow(ctx,
		`DELETE FROM second_factor WHERE account_id = $1 AND id = $2 RETURNING `+factorColumns, account, parsed))
}

func countFactors(ctx context.Context, tx pgx.Tx, account string) (int, error) {
	var n int
	err := tx.QueryRow(ctx, `SELECT count(*) FROM second_factor WHERE account_id = $1`, account).Scan(&n)
	return n, err
}

// replaceRecoveryCodes stores a fresh set in place of whatever the account
// had: generating new codes invalidates the old (spec, D6). Each hash is
// bound to the account (boundRecoveryHash) before it is stored: this is the
// only place that binds, so callers keep passing the plain recoveryHash(code).
func replaceRecoveryCodes(ctx context.Context, tx pgx.Tx, marketplace, account string, hashes [][]byte) error {
	if err := deleteRecoveryCodes(ctx, tx, account); err != nil {
		return err
	}
	for _, hash := range hashes {
		if _, err := tx.Exec(ctx, `
			INSERT INTO recovery_code (account_id, marketplace_id, code_hash) VALUES ($1, $2, $3)`,
			account, marketplace, boundRecoveryHash(account, hash)); err != nil {
			return err
		}
	}
	return nil
}

func deleteRecoveryCodes(ctx context.Context, tx pgx.Tx, account string) error {
	_, err := tx.Exec(ctx, `DELETE FROM recovery_code WHERE account_id = $1`, account)
	return err
}

// recoveryCodes reports how many codes an account was issued in its current
// set, and how many of them are left.
func recoveryCodes(ctx context.Context, tx pgx.Tx, account string) (issued, left int, err error) {
	err = tx.QueryRow(ctx, `
		SELECT count(*), count(*) FILTER (WHERE used_at IS NULL) FROM recovery_code WHERE account_id = $1`,
		account).Scan(&issued, &left)
	return issued, left, err
}

// enrolment is a factor_enrolment row.
type enrolment struct {
	Method          Method
	Secret          []byte
	WebAuthnSession []byte
}

// putEnrolment starts an enrolment for a session, replacing any the session
// had in progress.
func putEnrolment(ctx context.Context, tx pgx.Tx, marketplace, session, account string, method Method,
	secret, webauthnSession []byte, expires time.Time) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO factor_enrolment (session_id, account_id, marketplace_id, kind, secret, webauthn_session, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (session_id) DO UPDATE SET kind = EXCLUDED.kind, secret = EXCLUDED.secret,
			webauthn_session = EXCLUDED.webauthn_session, expires_at = EXCLUDED.expires_at, created_at = now()`,
		session, account, marketplace, method, secret, webauthnSession, expires)
	return err
}

// ErrNoEnrolment is a session with no enrolment of that kind in progress, or
// one that expired.
var ErrNoEnrolment = errors.New("identity: no enrolment in progress")

// enrolmentOf locks a session's enrolment of method, if it is still live.
// It locks the session's account first, then the enrolment row itself
// (lock order: account, then enrolment, everywhere), so that confirming an
// enrolment serialises with anything else that touches the same account's
// factors.
func enrolmentOf(ctx context.Context, tx pgx.Tx, session string, method Method, now time.Time) (enrolment, error) {
	if _, err := tx.Exec(ctx,
		`SELECT 1 FROM account WHERE id = (SELECT account_id FROM session WHERE id = $1) FOR NO KEY UPDATE`,
		session); err != nil {
		return enrolment{}, err
	}
	var e enrolment
	err := tx.QueryRow(ctx, `
		SELECT kind, secret, webauthn_session FROM factor_enrolment
		 WHERE session_id = $1 AND kind = $2 AND expires_at > $3 FOR UPDATE`, session, method, now).
		Scan(&e.Method, &e.Secret, &e.WebAuthnSession)
	if errors.Is(err, pgx.ErrNoRows) {
		return enrolment{}, ErrNoEnrolment
	}
	return e, err
}

func dropEnrolment(ctx context.Context, tx pgx.Tx, session string) error {
	_, err := tx.Exec(ctx, `DELETE FROM factor_enrolment WHERE session_id = $1`, session)
	return err
}
