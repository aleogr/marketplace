package identity

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

// challenge is a sign_in_challenge row.
type challenge struct {
	Account string
	// Session is the session a step-up steps up, empty at sign-in.
	Session         string
	Action          Action
	Attempts        int
	WebAuthnSession []byte
}

// insertChallenge opens a challenge for account: at sign-in with no session,
// or a step-up of session for action. The account's challenges that can no
// longer be answered go first, so the table holds a few rows per account at
// most.
func insertChallenge(ctx context.Context, tx pgx.Tx, marketplace, account string, hash []byte,
	session string, action Action, now time.Time) error {
	if _, err := tx.Exec(ctx, `
		DELETE FROM sign_in_challenge
		 WHERE account_id = $1 AND (expires_at <= $2 OR used_at IS NOT NULL)`, account, now); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO sign_in_challenge (token_hash, account_id, marketplace_id, session_id, action, expires_at)
		VALUES ($1, $2, $3, nullif($4, '')::uuid, nullif($5, ''), $6)`,
		hash, account, marketplace, session, string(action), now.Add(ChallengeLifetime))
	return err
}

// challengeForUpdate locks a challenge that can still be answered: not used,
// not expired and with attempts left. Any other is pgx.ErrNoRows.
func challengeForUpdate(ctx context.Context, tx pgx.Tx, hash []byte, now time.Time) (challenge, error) {
	var c challenge
	var action string
	err := tx.QueryRow(ctx, `
		SELECT account_id::text, coalesce(session_id::text, ''), coalesce(action, ''), attempts, webauthn_session
		  FROM sign_in_challenge
		 WHERE token_hash = $1 AND used_at IS NULL AND expires_at > $2 AND attempts < $3
		   FOR UPDATE`, hash, now, ChallengeAttempts).
		Scan(&c.Account, &c.Session, &action, &c.Attempts, &c.WebAuthnSession)
	c.Action = Action(action)
	return c, err
}

// challengeAttempt counts a wrong answer and returns how many there have
// been. The last one allowed spends the challenge.
func challengeAttempt(ctx context.Context, tx pgx.Tx, hash []byte, now time.Time) (int, error) {
	var attempts int
	err := tx.QueryRow(ctx, `
		UPDATE sign_in_challenge
		   SET attempts = attempts + 1,
		       used_at = CASE WHEN attempts + 1 >= $3 THEN $2 ELSE used_at END
		 WHERE token_hash = $1
		RETURNING attempts`, hash, now, ChallengeAttempts).Scan(&attempts)
	return attempts, err
}

// spendChallenge uses a challenge up: a right answer opens one session or one
// step-up, and never two.
func spendChallenge(ctx context.Context, tx pgx.Tx, hash []byte, now time.Time) error {
	_, err := tx.Exec(ctx, `UPDATE sign_in_challenge SET used_at = $2 WHERE token_hash = $1`, hash, now)
	return err
}

// challengeAddress is the normalised address of the account a challenge is
// for, whatever its state, or pgx.ErrNoRows.
func challengeAddress(ctx context.Context, tx pgx.Tx, hash []byte) (string, error) {
	var address string
	err := tx.QueryRow(ctx, `
		SELECT a.email_normalised FROM sign_in_challenge c JOIN account a ON a.id = c.account_id
		 WHERE c.token_hash = $1`, hash).Scan(&address)
	return address, err
}

// markSteppedUp records that a session proved a second factor at at.
func markSteppedUp(ctx context.Context, tx pgx.Tx, session string, at time.Time) error {
	_, err := tx.Exec(ctx, `UPDATE session SET stepped_up_at = $2 WHERE id = $1`, session, at)
	return err
}

// markEmailConfirmed records that a session answered a code sent to its
// account's address, which is not one of the account's factors, at at.
func markEmailConfirmed(ctx context.Context, tx pgx.Tx, session string, at time.Time) error {
	_, err := tx.Exec(ctx, `UPDATE session SET email_confirmed_at = $2 WHERE id = $1`, session, at)
	return err
}

// sessionID is the id of the session a token hash opened.
func sessionID(ctx context.Context, tx pgx.Tx, hash []byte) (string, error) {
	var id string
	err := tx.QueryRow(ctx, `SELECT id::text FROM session WHERE token_hash = $1`, hash).Scan(&id)
	return id, err
}

// appsForUpdate locks an account's authenticator apps, so two answers with
// the same code cannot both pass the replay check.
func appsForUpdate(ctx context.Context, tx pgx.Tx, account string) ([]factor, error) {
	rows, err := tx.Query(ctx, `
		SELECT `+factorColumns+` FROM second_factor
		 WHERE account_id = $1 AND kind = 'totp' ORDER BY created_at, id FOR UPDATE`, account)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (factor, error) { return scanFactor(row) })
}

// keysForUpdate locks an account's keys, so two answers from one key cannot
// both pass the counter check (D6), and a slower one cannot move the stored
// counter backwards. The lock order is the app's: the challenge's row first,
// then the factors'.
func keysForUpdate(ctx context.Context, tx pgx.Tx, account string) ([]factor, error) {
	rows, err := tx.Query(ctx, `
		SELECT `+factorColumns+` FROM second_factor
		 WHERE account_id = $1 AND kind = 'webauthn' ORDER BY created_at, id FOR UPDATE`, account)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (factor, error) { return scanFactor(row) })
}

// usedApp records the step an app's code was accepted for.
func usedApp(ctx context.Context, tx pgx.Tx, id string, step int64, now time.Time) error {
	_, err := tx.Exec(ctx,
		`UPDATE second_factor SET totp_last_step = $2, last_used_at = $3 WHERE id = $1`, id, step, now)
	return err
}

// useRecoveryCode spends a code, and reports false for one the account does
// not have or has already used: a code works exactly once. The stored
// code_hash is bound to the account (boundRecoveryHash), so hash is bound
// the same way before it is compared.
func useRecoveryCode(ctx context.Context, tx pgx.Tx, account string, hash []byte, now time.Time) (bool, error) {
	tag, err := tx.Exec(ctx, `
		UPDATE recovery_code SET used_at = $3
		 WHERE account_id = $1 AND code_hash = $2 AND used_at IS NULL`, account, boundRecoveryHash(account, hash), now)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}
