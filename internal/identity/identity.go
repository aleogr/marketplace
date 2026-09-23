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

// ErrTokenInvalid is a token that is unknown, spent or expired. One error for
// all three: which one it was is nobody's business but the log's.
var ErrTokenInvalid = errors.New("identity: token invalid")

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
// the same page either way (spec, D7). The password is hashed before the
// transaction opens, and for a taken address too, so both answers cost the
// same.
func (s *Service) SignUp(ctx context.Context, v Visit, name, email, password string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return ErrNameMissing
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
