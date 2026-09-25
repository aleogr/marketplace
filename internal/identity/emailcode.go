package identity

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"fmt"
	"math/big"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/aleogr/marketplace/internal/platform/mail"
)

// The codes sent by e-mail (F14 spec, D6): six digits, ten minutes, five
// attempts, one purpose each, and at most one sent a minute and five an hour
// per account, whatever their purpose.
const (
	EmailCodeLifetime = 10 * time.Minute
	EmailCodeAttempts = 5
	emailCodeEvery    = time.Minute
	emailCodesPerHour = 5
)

// A code's purpose: the second step of a sign-in, a step-up, or adding e-mail
// as a second factor. A code of one is never accepted for another.
const (
	purposeSignIn = "signin"
	purposeStepUp = "stepup"
	purposeEnrol  = "enrol"
)

var (
	// ErrCodeTooSoon is a code asked for less than a minute after the last
	// one sent to the account.
	ErrCodeTooSoon = errors.New("identity: a code was sent less than a minute ago")
	// ErrCodeTooMany is a sixth code asked for within an hour.
	ErrCodeTooMany = errors.New("identity: too many codes sent in the last hour")
	// ErrAlreadyEnrolled is adding e-mail to an account that has it.
	ErrAlreadyEnrolled = errors.New("identity: the account already has this method")
)

// newEmailCode returns six random digits and their SHA-256, which is all that
// is stored. A six-digit hash can be reversed by whoever holds the database;
// the code's ten minutes bound that, and its five attempts bound guessing
// online (spec, Known limit).
func newEmailCode() (string, []byte, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", nil, err
	}
	code := fmt.Sprintf("%06d", n.Int64())
	hash, _ := emailCodeHash(code)
	return code, hash, nil
}

// emailCodeHash is how a typed code is compared, spaces ignored; false for
// anything that is not six digits.
func emailCodeHash(typed string) ([]byte, bool) {
	code := strings.ReplaceAll(typed, " ", "")
	if len(code) != 6 || strings.Trim(code, "0123456789") != "" {
		return nil, false
	}
	sum := sha256.Sum256([]byte(code))
	return sum[:], true
}

// emailCodesSince counts the codes sent to an account since a moment.
func emailCodesSince(ctx context.Context, tx pgx.Tx, account string, since time.Time) (int, error) {
	var n int
	err := tx.QueryRow(ctx, `SELECT count(*) FROM email_code WHERE account_id = $1 AND created_at > $2`,
		account, since).Scan(&n)
	return n, err
}

// insertEmailCode stores a code for purpose, and retires the account's
// earlier codes of that purpose: the last one sent is the one that works.
// Codes older than the hourly limit's window are no longer counted, and go.
func insertEmailCode(ctx context.Context, tx pgx.Tx, marketplace, account, purpose string, hash []byte, now time.Time) error {
	if _, err := tx.Exec(ctx, `DELETE FROM email_code WHERE account_id = $1 AND created_at <= $2`,
		account, now.Add(-time.Hour)); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE email_code SET used_at = $3 WHERE account_id = $1 AND purpose = $2 AND used_at IS NULL`,
		account, purpose, now); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO email_code (account_id, marketplace_id, purpose, code_hash, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)`, account, marketplace, purpose, hash, now.Add(EmailCodeLifetime), now)
	return err
}

// useEmailCode spends the account's live code of purpose when typed is it,
// and counts an attempt against it when it is not: five, and it is dead.
func useEmailCode(ctx context.Context, tx pgx.Tx, account, purpose, typed string, now time.Time) (bool, error) {
	var id string
	var stored []byte
	err := tx.QueryRow(ctx, `
		SELECT id::text, code_hash FROM email_code
		 WHERE account_id = $1 AND purpose = $2 AND used_at IS NULL AND expires_at > $3 AND attempts < $4
		 ORDER BY created_at DESC LIMIT 1 FOR UPDATE`, account, purpose, now, EmailCodeAttempts).Scan(&id, &stored)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	hash, ok := emailCodeHash(typed)
	if ok && subtle.ConstantTimeCompare(hash, stored) == 1 {
		_, err := tx.Exec(ctx, `UPDATE email_code SET used_at = $2 WHERE id = $1`, id, now)
		if err == nil {
			// A code spent is the e-mail factor used, when e-mail is one, as
			// an app's code is (usedApp). The code's row is locked before the
			// factor's, and nothing locks the two the other way round.
			_, err = tx.Exec(ctx,
				`UPDATE second_factor SET last_used_at = $2 WHERE account_id = $1 AND kind = 'email'`, account, now)
		}
		return err == nil, err
	}
	_, err = tx.Exec(ctx, `UPDATE email_code SET attempts = attempts + 1 WHERE id = $1`, id)
	return false, err
}

// sendCode mails a fresh code for purpose to the account's address, within
// the per-account limits, through the outbox: its variables are cleared once
// it is dispatched (migrations/00010), so the code does not stay behind. The
// account is locked before the limits are counted, so two codes asked for at
// once, whatever their purpose or challenge, are counted one after the other
// rather than both against the same count. A challenge, when there is one,
// is locked before it (SendChallengeCode), and nothing locks the two the
// other way round.
func (s *Service) sendCode(ctx context.Context, tx pgx.Tx, v Visit, account Account, purpose string, now time.Time) error {
	if err := lockAccount(ctx, tx, account.ID); err != nil {
		return err
	}
	if n, err := emailCodesSince(ctx, tx, account.ID, now.Add(-emailCodeEvery)); err != nil || n > 0 {
		if err != nil {
			return err
		}
		return ErrCodeTooSoon
	}
	if n, err := emailCodesSince(ctx, tx, account.ID, now.Add(-time.Hour)); err != nil || n >= emailCodesPerHour {
		if err != nil {
			return err
		}
		return ErrCodeTooMany
	}
	code, hash, err := newEmailCode()
	if err != nil {
		return err
	}
	if err := insertEmailCode(ctx, tx, v.Marketplace, account.ID, purpose, hash, now); err != nil {
		return err
	}
	return mail.Request(ctx, tx, mail.Message{
		Template: "second-factor-code", Language: v.Language, To: account.Email,
		From: v.MarketplaceName, Marketplace: v.Marketplace,
		Variables: map[string]string{"Name": account.Name, "Code": code},
	})
}

// purposeOf is what a challenge's e-mail code is for.
func purposeOf(ch challenge) string {
	if ch.Session == "" {
		return purposeSignIn
	}
	return purposeStepUp
}

// SendChallengeCode mails a code that answers the challenge a token opened,
// for the session sessionID steps up, or for none at sign-in, when the
// challenge accepts one.
func (s *Service) SendChallengeCode(ctx context.Context, v Visit, token, sessionID string) error {
	return s.db.InTxFor(ctx, v.Marketplace, func(tx pgx.Tx) error {
		ch, account, err := s.challenge(ctx, tx, token, sessionID)
		if err != nil {
			return err
		}
		methods, _, err := s.offers(ctx, tx, account, ch)
		if err != nil {
			return err
		}
		if !slices.Contains(methods, MethodEmail) {
			return ErrNotPermitted
		}
		return s.sendCode(ctx, tx, v, account, purposeOf(ch), s.now())
	})
}

// canEnrolEmail is what adding e-mail asks first: the policy permits it (never
// for staff, §18.2), and a recent step-up when the account has 2FA (D2).
func (s *Service) canEnrolEmail(session Session) error {
	if !s.policy.Permits(session.Account.Kind, MethodEmail, Enrol) {
		return ErrNotPermitted
	}
	if s.NeedsStepUp(session, ActionFactors) {
		return ErrStepUpNeeded
	}
	return nil
}

// hasEmailFactor reports whether an account receives its codes by e-mail.
func hasEmailFactor(ctx context.Context, tx pgx.Tx, account string) (bool, error) {
	factors, err := factorsOf(ctx, tx, account)
	if err != nil {
		return false, err
	}
	return slices.ContainsFunc(factors, func(f factor) bool { return f.Method == MethodEmail }), nil
}

// BeginEmail sends the code that adds e-mail as the account's second factor:
// the method is added only when the code comes back, which proves the address
// receives it.
func (s *Service) BeginEmail(ctx context.Context, v Visit, session Session) error {
	if err := s.canEnrolEmail(session); err != nil {
		return err
	}
	return s.db.InTxFor(ctx, v.Marketplace, func(tx pgx.Tx) error {
		has, err := hasEmailFactor(ctx, tx, session.Account.ID)
		if err != nil {
			return err
		}
		if has {
			return ErrAlreadyEnrolled
		}
		return s.sendCode(ctx, tx, v, session.Account, purposeEnrol, s.now())
	})
}

// ConfirmEmail adds e-mail as a second factor once code is the one BeginEmail
// sent. It shows no recovery codes: those come with an app or a key (§18.2).
func (s *Service) ConfirmEmail(ctx context.Context, v Visit, session Session, code string) error {
	if err := s.canEnrolEmail(session); err != nil {
		return err
	}
	account := session.Account.ID
	var wrong bool
	err := s.db.InTxFor(ctx, v.Marketplace, func(tx pgx.Tx) error {
		if err := lockAccount(ctx, tx, account); err != nil {
			return err
		}
		has, err := hasEmailFactor(ctx, tx, account)
		if err != nil {
			return err
		}
		if has {
			return ErrAlreadyEnrolled
		}
		now := s.now()
		ok, err := useEmailCode(ctx, tx, account, purposeEnrol, code, now)
		if err != nil {
			return err
		}
		if !ok {
			wrong = true
			return nil
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO second_factor (account_id, marketplace_id, kind, label, created_at)
			VALUES ($1, $2, 'email', '', $3)`, account, v.Marketplace, now); err != nil {
			return err
		}
		return s.recordWith(ctx, tx, v, account, "identity.second_factor_added", map[string]string{"method": string(MethodEmail)})
	})
	if err == nil && wrong {
		return ErrCodeWrong
	}
	return err
}
