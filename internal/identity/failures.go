package identity

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
)

// The consecutive failed second-factor answers of an account are counted
// (F14 spec, D8; the owner's decision of 2026-09-25). Someone who has the
// password is otherwise bounded only by the per-address sign-in limit, about
// 770 guesses a day at a six-digit code, and the owner never knows. At
// FailuresNotified the owner is told by e-mail; at FailuresLocked, the cap
// NIST SP 800-63B §5.2.2 sets on consecutive failures, the codes an app
// shows or an e-mail carries are refused, at sign-in and at a step-up, until
// the password is changed. A key and a recovery code still answer, and a
// right answer with either, like any right second factor, ends the run.
const (
	FailuresNotified = 10
	FailuresLocked   = 100
)

// ErrCodesLocked is an answer with a code — an app's or an e-mail's — to an
// account whose codes are locked (D8): refused without being checked; and a
// code asked for by e-mail for such an account, which is not sent. It is an
// ErrCodeWrong too, for a caller that makes no difference.
var ErrCodesLocked = fmt.Errorf("%w: codes are locked", ErrCodeWrong)

// locks reports whether the account's lock refuses method: while its codes
// are locked, a code a person reads and types, which can be guessed. A key's
// answer cannot, and a recovery code's sixty bits are not guessed in a
// hundred tries.
func (o offer) locks(method Method) bool {
	return o.CodesLocked && (method == MethodApp || method == MethodEmail)
}

// secondFactor reports whether an answer with method is an answer with one
// of the account's second factors, which the count is of: a key, an app, a
// recovery code, or an e-mail code when e-mail is one of its factors. A code
// sent to the account's address for an action that accepts one, e-mail not
// being a factor, proves the address and not a second factor (D4): it
// neither counts when wrong nor ends a run when right.
func (o offer) secondFactor(method Method) bool {
	switch method {
	case MethodKey, MethodApp, MethodRecovery:
		return true
	case MethodEmail:
		return o.EmailFactor
	}
	return false
}

// failed counts a failed second-factor answer, and tells the owner, and
// audits, when the run reaches a count that matters: exactly FailuresNotified
// and exactly FailuresLocked, so each is sent once a run. Like the failure
// itself it is recorded with the platform as the actor: whoever failed is
// not known to be the account's owner.
func (s *Service) failed(ctx context.Context, tx pgx.Tx, v Visit, account Account, now time.Time) error {
	failures, err := countFailure(ctx, tx, v.Marketplace, account.ID, now)
	if err != nil {
		return err
	}
	var action, template string
	switch failures {
	case FailuresNotified:
		action, template = "identity.second_factor_failures_notified", "second-factor-failures"
	case FailuresLocked:
		action, template = "identity.second_factor_locked", "second-factor-locked"
	default:
		return nil
	}
	if err := s.refusalWith(ctx, tx, v, account.ID, action,
		map[string]string{"failures": strconv.Itoa(failures)}); err != nil {
		return err
	}
	return notify(ctx, tx, v, account, template, nil)
}

// secondFactorFailures is how many second-factor answers in a row the
// account has failed: what a challenge offers. It is read without a lock,
// which would come before the factor rows' and break the lock order, so an
// answer already being checked when another's failure reaches
// FailuresLocked is still checked; each needs a challenge of its own, which
// needs the password and the sign-in limit, so they are few.
func secondFactorFailures(ctx context.Context, tx pgx.Tx, account string) (int, error) {
	var failures int
	err := tx.QueryRow(ctx, `
		SELECT coalesce((SELECT failures FROM second_factor_failure WHERE account_id = $1), 0)`,
		account).Scan(&failures)
	return failures, err
}

// countFailure adds one to the account's run of failures and returns it. The
// upsert is one statement, so two failures at once count two: the second
// waits for the first's row, even one the first is creating, and adds to it.
//
// Lock order: the row is the last one any transaction takes. An answer takes
// its challenge, then the factor rows it checks, then the audit chain, then
// this row; a right answer and a password change take it last too
// (clearFailures). After it a transaction only audits, under the chain it
// already holds, and inserts mail into the outbox: it waits for no other.
func countFailure(ctx context.Context, tx pgx.Tx, marketplace, account string, now time.Time) (int, error) {
	var failures int
	err := tx.QueryRow(ctx, `
		INSERT INTO second_factor_failure (account_id, marketplace_id, failures, updated_at)
		VALUES ($1, $2, 1, $3)
		ON CONFLICT (account_id) DO UPDATE
		   SET failures = second_factor_failure.failures + 1, updated_at = EXCLUDED.updated_at
		RETURNING failures`, account, marketplace, now).Scan(&failures)
	return failures, err
}

// clearFailures ends the account's run of failures, and with it the lock:
// a right second factor, or a new password. It takes the row last, as
// countFailure does.
func clearFailures(ctx context.Context, tx pgx.Tx, account string) error {
	_, err := tx.Exec(ctx, `DELETE FROM second_factor_failure WHERE account_id = $1`, account)
	return err
}
