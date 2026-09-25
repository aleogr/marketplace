package identity

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
)

// The challenge a correct password opens on an account with a second factor
// (F14 spec, D3): five minutes, five attempts, used once.
const (
	ChallengeLifetime = 5 * time.Minute
	ChallengeAttempts = 5
)

// MethodRecovery answers a challenge with a recovery code. It is not a kind
// of second factor, and nothing enrols it.
const MethodRecovery Method = "recovery"

var (
	// ErrSecondStep is not a refusal: the password was right and the account
	// has a second factor, so the token SignIn returned with it opens a
	// challenge, not a session (D3). A caller that treats it as any other
	// error signs nobody in.
	ErrSecondStep = errors.New("identity: the second step is needed")
	// ErrChallengeInvalid is a challenge that is unknown, expired, used or
	// another session's: the person starts again.
	ErrChallengeInvalid = errors.New("identity: challenge invalid")
	// ErrChallengeExhausted is the wrong answer that used a challenge's last
	// attempt: the person starts again.
	ErrChallengeExhausted = errors.New("identity: challenge attempts exhausted")
)

// Answer is what a person gives at the second step: the method, and the code
// it produced or, for a key, its answer as the browser's
// navigator.credentials.get gave it, in JSON.
type Answer struct {
	Method Method
	Code   string
	Key    []byte
}

// Pending is a challenge waiting for its answer, as its page shows it.
type Pending struct {
	// Methods are what the account may answer with, strongest first.
	Methods []Method
	// Recovery is whether a recovery code may answer: the account has unused
	// ones.
	Recovery bool
	// Action is what a step-up guards; empty at sign-in.
	Action Action
}

// offers returns the methods a challenge accepts, strongest first: the
// account's own second factors that the policy permits it to verify with,
// and, for a step-up whose action accepts it (§18.2), a code to the account's
// address even with no e-mail factor enrolled.
func (s *Service) offers(ctx context.Context, tx pgx.Tx, account Account, ch challenge) ([]Method, bool, error) {
	factors, err := factorsOf(ctx, tx, account.ID)
	if err != nil {
		return nil, false, err
	}
	var methods []Method
	for _, f := range factors {
		if s.policy.Permits(account.Kind, f.Method, Verify) {
			methods = append(methods, f.Method)
		}
	}
	if ch.Action != "" && s.policy.StepUpFor(account.Kind, ch.Action, len(factors) > 0).Email {
		methods = append(methods, MethodEmail)
	}
	_, left, err := recoveryCodes(ctx, tx, account.ID)
	return Strongest(methods), left > 0, err
}

// Pending reads the challenge a token opened, for the session sessionID steps
// up, or for none at sign-in; ErrChallengeInvalid for one that cannot be
// answered.
func (s *Service) Pending(ctx context.Context, v Visit, token, sessionID string) (Pending, error) {
	var pending Pending
	err := s.db.InTxFor(ctx, v.Marketplace, func(tx pgx.Tx) error {
		ch, account, err := s.challenge(ctx, tx, token, sessionID)
		if err != nil {
			return err
		}
		pending.Action = ch.Action
		pending.Methods, pending.Recovery, err = s.offers(ctx, tx, account, ch)
		return err
	})
	return pending, err
}

// challenge locks the live challenge a token opened, and reads its account.
// A challenge is answered only by the session it names, or at sign-in when it
// names none.
func (s *Service) challenge(ctx context.Context, tx pgx.Tx, token, sessionID string) (challenge, Account, error) {
	ch, err := challengeForUpdate(ctx, tx, HashToken(token), s.now())
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && ch.Session != sessionID) {
		return challenge{}, Account{}, ErrChallengeInvalid
	}
	if err != nil {
		return challenge{}, Account{}, err
	}
	account, err := accountByID(ctx, tx, ch.Account)
	return ch, account, err
}

// ChallengeAddress is the normalised address of the account a challenge is
// for, whatever the challenge's state, or "" for a token that opened none. It
// is what the per-address sign-in limit counts the second step against, so
// the password and the second step share one limit.
func (s *Service) ChallengeAddress(ctx context.Context, v Visit, token string) string {
	var address string
	if err := s.db.InTxFor(ctx, v.Marketplace, func(tx pgx.Tx) error {
		var err error
		address, err = challengeAddress(ctx, tx, HashToken(token))
		return err
	}); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		s.log.WarnContext(ctx, "a challenge's address could not be read", "error", err)
	}
	return address
}

// check reports whether answer proves the account's second factor, spending
// what it used: an app's step, a recovery code.
func (s *Service) check(ctx context.Context, tx pgx.Tx, v Visit, account Account, ch challenge, answer Answer, now time.Time) (bool, error) {
	methods, recovery, err := s.offers(ctx, tx, account, ch)
	if err != nil {
		return false, err
	}
	if answer.Method == MethodRecovery {
		if !recovery {
			return false, nil
		}
		hash, ok := recoveryHash(answer.Code)
		if !ok {
			return false, nil
		}
		return useRecoveryCode(ctx, tx, account.ID, hash, now)
	}
	if !slices.Contains(methods, answer.Method) {
		return false, nil
	}
	switch answer.Method {
	case MethodApp:
		return s.checkApp(ctx, tx, account.ID, answer.Code, now)
	case MethodEmail:
		return useEmailCode(ctx, tx, account.ID, purposeOf(ch), answer.Code, now)
	case MethodKey:
		return s.checkKey(ctx, tx, v, account, ch, answer.Key, now)
	}
	return false, nil
}

// checkApp accepts a code from any of the account's apps, once.
func (s *Service) checkApp(ctx context.Context, tx pgx.Tx, account, code string, now time.Time) (bool, error) {
	if s.sealer == nil {
		return false, errNoSealer
	}
	apps, err := appsForUpdate(ctx, tx, account)
	if err != nil {
		return false, err
	}
	for _, app := range apps {
		secret, err := s.sealer.OpenFor(ctx, tx, account, app.Secret)
		if err != nil {
			return false, err
		}
		var last int64
		if app.LastStep != nil {
			last = *app.LastStep
		}
		if step, ok := matchTOTP(secret, code, now, last); ok {
			return true, usedApp(ctx, tx, app.ID, step, now)
		}
	}
	return false, nil
}

// answerChallenge runs one answer against the challenge a token opened, for
// sessionID ("" at sign-in). A wrong answer counts an attempt, is audited, and
// is ErrCodeWrong, or ErrChallengeExhausted when it was the last; a right one
// spends the challenge and runs done in the same transaction.
func (s *Service) answerChallenge(ctx context.Context, v Visit, token, sessionID string, answer Answer,
	done func(tx pgx.Tx, ch challenge, account Account, now time.Time) error) error {
	var wrong, exhausted bool
	err := s.db.InTxFor(ctx, v.Marketplace, func(tx pgx.Tx) error {
		now := s.now()
		ch, account, err := s.challenge(ctx, tx, token, sessionID)
		if err != nil {
			return err
		}
		ok, err := s.check(ctx, tx, v, account, ch, answer, now)
		if err != nil {
			return err
		}
		if !ok {
			wrong = true
			attempts, err := challengeAttempt(ctx, tx, HashToken(token), now)
			if err != nil {
				return err
			}
			exhausted = attempts >= ChallengeAttempts
			return s.refusalWith(ctx, tx, v, account.ID, "identity.second_factor_failed",
				map[string]string{"method": string(answer.Method)})
		}
		if err := spendChallenge(ctx, tx, HashToken(token), now); err != nil {
			return err
		}
		if answer.Method == MethodRecovery {
			if err := s.recoveryUsed(ctx, tx, v, account); err != nil {
				return err
			}
		}
		return done(tx, ch, account, now)
	})
	switch {
	case err != nil:
		return err
	case exhausted:
		return ErrChallengeExhausted
	case wrong:
		return ErrCodeWrong
	}
	return nil
}

// recoveryUsed audits a recovery code spent, with how many are left.
func (s *Service) recoveryUsed(ctx context.Context, tx pgx.Tx, v Visit, account Account) error {
	_, left, err := recoveryCodes(ctx, tx, account.ID)
	if err != nil {
		return err
	}
	return s.recordWith(ctx, tx, v, account.ID, "identity.recovery_code_used",
		map[string]string{"left": strconv.Itoa(left)})
}

// SignedIn is what a completed second step opened: the session's token, and,
// when a recovery code answered it, how many codes are left.
type SignedIn struct {
	Session      string
	Recovery     bool
	RecoveryLeft int
}

// CompleteSignIn answers the challenge a correct password opened. A right
// answer opens the session exactly as a sign-in without a second factor does
// (a new token, which the caller pairs with a new CSRF secret), stepped up
// from the start (D4), and audits which method answered.
func (s *Service) CompleteSignIn(ctx context.Context, v Visit, token string, answer Answer) (SignedIn, error) {
	session, hash, err := NewToken()
	if err != nil {
		return SignedIn{}, err
	}
	signed := SignedIn{Session: session, Recovery: answer.Method == MethodRecovery}
	err = s.answerChallenge(ctx, v, token, "", answer, func(tx pgx.Tx, _ challenge, account Account, now time.Time) error {
		if err := insertSession(ctx, tx, v.Marketplace, account.ID, hash, v.IP, v.UserAgent, now); err != nil {
			return err
		}
		id, err := sessionID(ctx, tx, hash)
		if err != nil {
			return err
		}
		if err := markSteppedUp(ctx, tx, id, now); err != nil {
			return err
		}
		if signed.Recovery {
			if _, signed.RecoveryLeft, err = recoveryCodes(ctx, tx, account.ID); err != nil {
				return err
			}
		}
		return s.recordWith(ctx, tx, v, account.ID, "identity.signin", map[string]string{"method": string(answer.Method)})
	})
	if err != nil {
		return SignedIn{}, err
	}
	s.sweep(ctx, v.Marketplace)
	return signed, nil
}
