package identity

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/aleogr/marketplace/internal/identity/breached"
	"github.com/aleogr/marketplace/internal/platform/audit"
	"github.com/aleogr/marketplace/internal/platform/mail"
)

// VerificationLifetime is how long a confirmation link works (spec).
const VerificationLifetime = 24 * time.Hour

// Session lengths (spec, D3): the owner's choice of 2026-09-23. Constants now,
// parameters in F17. last_seen_at is written at most once every touchEvery, so
// a click does not cost a write.
const (
	SessionIdle     = 30 * 24 * time.Hour
	SessionLifetime = 90 * 24 * time.Hour
	touchEvery      = time.Hour
)

var (
	// ErrTokenInvalid is a token that is unknown, spent or expired. One error
	// for all three: which one it was is nobody's business but the log's.
	ErrTokenInvalid = errors.New("identity: token invalid")
	// ErrCredentials is a wrong password and an unknown address alike (D7).
	ErrCredentials = errors.New("identity: credentials refused")
	// ErrUnverified is a right password on an unconfirmed account: the one
	// case where the answer says more than "refused", because only the owner
	// of the password can reach it.
	ErrUnverified = errors.New("identity: address not confirmed")
	// ErrSessionInvalid is a session that is unknown, revoked, idle or expired.
	ErrSessionInvalid = errors.New("identity: session invalid")
)

// Transactor opens a transaction for one marketplace (db.Pool.InTxFor).
type Transactor interface {
	InTxFor(ctx context.Context, marketplaceID string, fn func(pgx.Tx) error) error
}

// Auditor appends to the audit log ((*audit.Log).Append).
type Auditor interface {
	Append(ctx context.Context, tx pgx.Tx, entry audit.Entry) error
}

// Visit is the request a flow runs for, as the flow needs it.
type Visit struct {
	Marketplace     string // id
	MarketplaceName string
	Language        string
	IP              string
	UserAgent       string
	BaseURL         string // scheme and host, e.g. https://m1.example
}

func (v Visit) link(path string, query url.Values) string {
	u := v.BaseURL + "/" + v.Language + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	return u
}

// Service runs the identity flows.
//
// No flow hashes or verifies a password inside a database transaction: a
// flow reads in one transaction, runs argon2 outside any, and writes in a
// second one (spec, D5; internal/platform/db).
type Service struct {
	db       Transactor
	hasher   *Hasher
	breached breached.Checker
	audit    Auditor
	log      *slog.Logger
	now      func() time.Time
}

// NewService returns the flows.
func NewService(db Transactor, hasher *Hasher, checker breached.Checker, auditor Auditor, log *slog.Logger) *Service {
	return &Service{db: db, hasher: hasher, breached: checker, audit: auditor, log: log,
		now: func() time.Time { return time.Now().UTC() }}
}

// checkNew applies D4 to a password about to be set.
func (s *Service) checkNew(ctx context.Context, password string) error {
	if err := CheckPassword(password); err != nil {
		return err
	}
	found, err := s.breached.Breached(ctx, normalise(password))
	if err != nil {
		// Fail open (spec, D4): a third party being down must not stop sign-up.
		s.log.WarnContext(ctx, "the breached-password check could not run", "error", err)
		return nil
	}
	if found {
		return ErrPasswordBreached
	}
	return nil
}

// record audits what an account did itself.
func (s *Service) record(ctx context.Context, tx pgx.Tx, v Visit, account, action string) error {
	after, err := json.Marshal(map[string]string{"user_agent": v.UserAgent})
	if err != nil {
		return err
	}
	return s.audit.Append(ctx, tx, audit.Entry{
		Marketplace: v.Marketplace,
		Actor:       audit.Actor{ID: account, Kind: audit.Buyer},
		Action:      action,
		Subject:     audit.Subject{Kind: "account", ID: account, Person: account},
		From:        v.IP,
		After:       after,
	})
}

// SignUp creates an unconfirmed account and sends the confirmation, or, for an
// address that already has an account, tells its owner so. The caller answers
// the same page either way (spec, D7). The dominant cost is equalised: the
// breached check and the argon2 hash run before the transaction opens, for a
// taken address too. What remains different is smaller and inside the
// transaction: a new account also writes its credential and its verification
// and appends to the audit log, which may create and wrap the person's key,
// where a taken address reads the existing account instead.
func (s *Service) SignUp(ctx context.Context, v Visit, name, email, password string) error {
	name, err := CheckName(name)
	if err != nil {
		return err
	}
	normalised, err := NormaliseEmail(email)
	if err != nil {
		return err
	}
	if err := s.checkNew(ctx, password); err != nil {
		return err
	}
	secret, err := s.hasher.Hash(ctx, password)
	if err != nil {
		return err
	}
	token, hash, err := NewToken()
	if err != nil {
		return err
	}

	return s.db.InTxFor(ctx, v.Marketplace, func(tx pgx.Tx) error {
		id, err := insertAccount(ctx, tx, v.Marketplace, strings.TrimSpace(email), normalised, name)
		if errors.Is(err, errTaken) {
			existing, err := accountByEmail(ctx, tx, normalised)
			if err != nil {
				return err
			}
			return mail.Request(ctx, tx, mail.Message{
				Template: "account-exists", Language: v.Language, To: existing.Email,
				From: v.MarketplaceName, Marketplace: v.Marketplace,
				Variables: map[string]string{"Name": existing.Name, "SignIn": v.link("/signin", nil)},
			})
		}
		if err != nil {
			return err
		}
		if err := setPassword(ctx, tx, v.Marketplace, id, secret); err != nil {
			return err
		}
		if err := insertVerification(ctx, tx, v.Marketplace, id, hash, s.now().Add(VerificationLifetime)); err != nil {
			return err
		}
		if err := mail.Request(ctx, tx, mail.Message{
			Template: "verify-email", Language: v.Language, To: strings.TrimSpace(email),
			From: v.MarketplaceName, Marketplace: v.Marketplace,
			Variables: map[string]string{"Name": name, "Link": v.link("/verify", url.Values{"token": {token}})},
		}); err != nil {
			return err
		}
		return s.record(ctx, tx, v, id, "identity.signup")
	})
}

// Verify spends a confirmation token.
func (s *Service) Verify(ctx context.Context, v Visit, token string) error {
	return s.db.InTxFor(ctx, v.Marketplace, func(tx pgx.Tx) error {
		account, err := consumeVerification(ctx, tx, HashToken(token), s.now())
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrTokenInvalid
		}
		if err != nil {
			return err
		}
		return s.record(ctx, tx, v, account, "identity.email_verified")
	})
}

// Resend issues a new confirmation to an unconfirmed address, and does nothing
// otherwise. The caller answers the same page either way (spec, D7).
func (s *Service) Resend(ctx context.Context, v Visit, email string) error {
	normalised, err := NormaliseEmail(email)
	if err != nil {
		return nil
	}
	token, hash, err := NewToken()
	if err != nil {
		return err
	}
	return s.db.InTxFor(ctx, v.Marketplace, func(tx pgx.Tx) error {
		account, err := accountByEmail(ctx, tx, normalised)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && account.VerifiedAt != nil) {
			return nil
		}
		if err != nil {
			return err
		}
		if err := insertVerification(ctx, tx, v.Marketplace, account.ID, hash, s.now().Add(VerificationLifetime)); err != nil {
			return err
		}
		return mail.Request(ctx, tx, mail.Message{
			Template: "verify-email", Language: v.Language, To: account.Email,
			From: v.MarketplaceName, Marketplace: v.Marketplace,
			Variables: map[string]string{"Name": account.Name, "Link": v.link("/verify", url.Values{"token": {token}})},
		})
	})
}

// Session is a signed-in request's account.
type Session struct {
	ID      string
	Account Account
}

// refusal audits an attempt against an account by somebody not known to be
// its owner. The platform is the actor and the account only the subject, not
// the person the record is about: the address the attempt came from is then
// sealed under the platform's key, so the owner's erasure request cannot
// erase the evidence about somebody else (internal/platform/audit).
func (s *Service) refusal(ctx context.Context, tx pgx.Tx, v Visit, account, action string) error {
	after, err := json.Marshal(map[string]string{"user_agent": v.UserAgent})
	if err != nil {
		return err
	}
	return s.audit.Append(ctx, tx, audit.Entry{
		Marketplace: v.Marketplace,
		Actor:       audit.Actor{Kind: audit.System},
		Action:      action,
		Subject:     audit.Subject{Kind: "account", ID: account},
		From:        v.IP,
		After:       after,
	})
}

// credentials reads an account and its password hash, or reports found false
// for an address with no account in this marketplace.
func (s *Service) credentials(ctx context.Context, marketplace, normalised string) (account Account, secret string, found bool, err error) {
	err = s.db.InTxFor(ctx, marketplace, func(tx pgx.Tx) error {
		a, err := accountByEmail(ctx, tx, normalised)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		account, found = a, true
		secret, err = passwordOf(ctx, tx, a.ID)
		return err
	})
	return account, secret, found, err
}

// SignIn checks a password and opens a new session, returning its token. A
// session is never reused: each sign-in opens its own.
//
// Three steps, and argon2 runs in none of the transactions: the account is
// read in one, the password verified with none open, and the session written
// in a second, which first checks that the password it verified is still the
// account's (spec, D5).
func (s *Service) SignIn(ctx context.Context, v Visit, email, password string) (string, error) {
	if len(password) > maxPasswordBytes {
		// No password that can be set is this long, so it is refused before
		// it costs a normalisation, a hash or a transaction. What that answer
		// takes tells nobody anything about the address.
		s.log.InfoContext(ctx, "sign-in with a password beyond the byte cap")
		return "", ErrCredentials
	}
	normalised, err := NormaliseEmail(email)
	if err != nil {
		s.hasher.Waste(ctx, password)
		return "", ErrCredentials
	}
	account, secret, found, err := s.credentials(ctx, v.Marketplace, normalised)
	if err != nil {
		return "", err
	}
	if !found {
		// The same work as a wrong password, so the two take as long (D7).
		s.hasher.Waste(ctx, password)
		s.log.InfoContext(ctx, "sign-in for an unknown address")
		return "", ErrCredentials
	}

	ok, stale, err := s.hasher.Verify(ctx, password, secret)
	if err != nil {
		return "", err
	}
	if !ok {
		if err := s.db.InTxFor(ctx, v.Marketplace, func(tx pgx.Tx) error {
			return s.refusal(ctx, tx, v, account.ID, "identity.signin_failed")
		}); err != nil {
			return "", err
		}
		return "", ErrCredentials
	}
	if account.VerifiedAt == nil {
		return "", ErrUnverified
	}

	// A hash made with other parameters is made again now, while the password
	// is at hand (spec, D5).
	var fresh string
	if stale {
		if fresh, err = s.hasher.Hash(ctx, password); err != nil {
			return "", err
		}
	}
	token, hash, err := NewToken()
	if err != nil {
		return "", err
	}
	err = s.db.InTxFor(ctx, v.Marketplace, func(tx pgx.Tx) error {
		still, err := passwordStill(ctx, tx, account.ID, secret)
		if err != nil {
			return err
		}
		if !still {
			// Changed between the read and now: what was verified is no
			// longer the password.
			return ErrCredentials
		}
		if fresh != "" {
			if err := setPassword(ctx, tx, v.Marketplace, account.ID, fresh); err != nil {
				return err
			}
		}
		if err := insertSession(ctx, tx, v.Marketplace, account.ID, hash, v.IP, v.UserAgent); err != nil {
			return err
		}
		return s.record(ctx, tx, v, account.ID, "identity.signin")
	})
	if err != nil {
		return "", err
	}
	return token, nil
}

// SignOut revokes the session a token opened. A token that opens no live
// session is not an error: there is nothing left to end.
func (s *Service) SignOut(ctx context.Context, v Visit, token string) error {
	return s.db.InTxFor(ctx, v.Marketplace, func(tx pgx.Tx) error {
		account, err := revokeSession(ctx, tx, HashToken(token), "signout", s.now())
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		return s.record(ctx, tx, v, account, "identity.signout")
	})
}

// Authenticate returns the session a token belongs to in marketplace, or
// ErrSessionInvalid for one that is unknown, revoked, idle or expired.
func (s *Service) Authenticate(ctx context.Context, marketplace, token string) (Session, error) {
	var session Session
	err := s.db.InTxFor(ctx, marketplace, func(tx pgx.Tx) error {
		now := s.now()
		row, err := liveSession(ctx, tx, HashToken(token))
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrSessionInvalid
		}
		if err != nil {
			return err
		}
		if now.Sub(row.seen) > SessionIdle || now.Sub(row.created) > SessionLifetime {
			return ErrSessionInvalid
		}
		if now.Sub(row.seen) > touchEvery {
			if err := touchSession(ctx, tx, row.id, now); err != nil {
				return err
			}
		}
		account, err := accountByID(ctx, tx, row.account)
		if err != nil {
			return err
		}
		session = Session{ID: row.id, Account: account}
		return nil
	})
	return session, err
}
