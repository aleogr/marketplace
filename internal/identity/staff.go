package identity

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
)

// Staff are accounts without a marketplace, served by the same flows as
// everybody else's on a platform visit (F15 spec, D4): passwords, sessions,
// second factors, the step-up and the failure counter are F13's and F14's.
// What is theirs alone is how an account comes to exist, which is never a
// public form.

// ErrAddressTaken is an address that already has a staff account.
var ErrAddressTaken = errors.New("identity: address taken")

// StaffHook is what the creation of a staff account runs in its own
// transaction, given the new account's id: the bootstrap's claim of the owner
// role, an invitation's use. An error from it rolls the account back.
type StaffHook func(ctx context.Context, tx pgx.Tx, account string) error

// CreateStaff creates a staff account on a platform visit, with a password
// that follows F13's rules, breach check included; runs then, when given, in
// the same transaction; and signs the account in, returning the session's
// token. The session has proved no second factor, since the account has
// none: the console sends it to enrol one before anything else (D4).
//
// As SignUp does, it hashes before the transaction opens, never inside it
// (F13 spec, D5).
func (s *Service) CreateStaff(ctx context.Context, v Visit, name, email, password string, then StaffHook) (string, error) {
	if !v.Platform {
		return "", errVisitScope
	}
	name, err := CheckName(name)
	if err != nil {
		return "", err
	}
	normalised, err := NormaliseEmail(email)
	if err != nil {
		return "", err
	}
	if err := s.checkNew(ctx, password); err != nil {
		return "", err
	}
	secret, err := s.hasher.Hash(ctx, password)
	if err != nil {
		return "", err
	}
	token, hash, err := NewToken()
	if err != nil {
		return "", err
	}
	err = s.within(ctx, v, func(tx pgx.Tx) error {
		now := s.now()
		id, err := insertStaff(ctx, tx, strings.TrimSpace(email), normalised, name, now)
		if errors.Is(err, errTaken) {
			return ErrAddressTaken
		}
		if err != nil {
			return err
		}
		if err := setPassword(ctx, tx, "", id, secret); err != nil {
			return err
		}
		if then != nil {
			if err := then(ctx, tx, id); err != nil {
				return err
			}
		}
		if err := s.record(ctx, tx, v, id, "identity.staff_created"); err != nil {
			return err
		}
		if err := insertSession(ctx, tx, "", id, hash, v.IP, v.UserAgent, now); err != nil {
			return err
		}
		return s.record(ctx, tx, v, id, "identity.signin")
	})
	if err != nil {
		return "", err
	}
	return token, nil
}
