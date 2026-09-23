//go:build integration

package identity

import (
	"bytes"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestChangingThePasswordEndsEveryOtherSession(t *testing.T) {
	s, db, one, _, trail := service(t)
	confirmed(t, s, db, one, "r@example.test")
	here := signIn(t, s, one, "r@example.test", "correct horse battery staple")
	there := signIn(t, s, one, "r@example.test", "correct horse battery staple")
	session, err := s.Authenticate(t.Context(), one, here)
	if err != nil {
		t.Fatal(err)
	}

	if err := s.ChangePassword(t.Context(), visit(one), session, "wrong current password", "a brand new passphrase"); !errors.Is(err, ErrCredentials) {
		t.Fatalf("wrong current: %v, want ErrCredentials", err)
	}
	if err := s.ChangePassword(t.Context(), visit(one), session, "correct horse battery staple", "password1234"); !errors.Is(err, ErrPasswordBreached) {
		t.Fatalf("breached new: %v, want ErrPasswordBreached", err)
	}
	if slices.Contains(trail.actions(), "identity.password_changed") {
		t.Fatalf("a refused change was audited as done: %v", trail.actions())
	}

	if err := s.ChangePassword(t.Context(), visit(one), session, "correct horse battery staple", "a brand new passphrase"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Authenticate(t.Context(), one, here); err != nil {
		t.Fatalf("the session that changed the password ended: %v", err)
	}
	if _, err := s.Authenticate(t.Context(), one, there); !errors.Is(err, ErrSessionInvalid) {
		t.Fatalf("the other session survived: %v", err)
	}
	if _, err := s.SignIn(t.Context(), visit(one), "r@example.test", "correct horse battery staple"); !errors.Is(err, ErrCredentials) {
		t.Fatalf("the old password still signs in: %v", err)
	}
	signIn(t, s, one, "r@example.test", "a brand new passphrase")

	mails := outbox(t, db, one)
	if mails[len(mails)-1].template != "password-changed" {
		t.Fatalf("no password-changed mail: %v", mails)
	}
	if !slices.Contains(trail.actions(), "identity.password_changed") {
		t.Fatalf("the change was not audited: %v", trail.actions())
	}
	changed := trail.entry(t, "identity.password_changed")
	if changed.Actor.ID != session.Account.ID || changed.Subject.Person != session.Account.ID {
		t.Errorf("password_changed recorded as %+v", changed)
	}

	var reason string
	if err := db.InTxFor(t.Context(), one, func(tx pgx.Tx) error {
		return tx.QueryRow(t.Context(), `SELECT revoked_reason FROM session WHERE token_hash = $1`,
			HashToken(there)).Scan(&reason)
	}); err != nil {
		t.Fatal(err)
	}
	if reason != "password_changed" {
		t.Errorf("revoked_reason = %q, want password_changed", reason)
	}
}

// A password change ends the sessions of its own account only.
func TestChangingThePasswordLeavesOtherAccountsSignedIn(t *testing.T) {
	s, db, one, _, _ := service(t)
	confirmed(t, s, db, one, "r@example.test")
	confirmed(t, s, db, one, "q@example.test")
	mine := signIn(t, s, one, "r@example.test", "correct horse battery staple")
	theirs := signIn(t, s, one, "q@example.test", "correct horse battery staple")
	session, err := s.Authenticate(t.Context(), one, mine)
	if err != nil {
		t.Fatal(err)
	}

	if err := s.ChangePassword(t.Context(), visit(one), session, "correct horse battery staple", "a brand new passphrase"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Authenticate(t.Context(), one, theirs); err != nil {
		t.Fatalf("another account's session ended: %v", err)
	}
}

// A current password whose hash changed while it was being verified — a
// concurrent change, or a sign-in's rehash — is refused like a wrong one, and
// nothing is written: the other sessions live on and nothing is audited.
func TestAPasswordChangedMidChangeIsRefused(t *testing.T) {
	s, db, one, _, _ := service(t)
	confirmed(t, s, db, one, "r@example.test")
	here := signIn(t, s, one, "r@example.test", "correct horse battery staple")
	there := signIn(t, s, one, "r@example.test", "correct horse battery staple")
	session, err := s.Authenticate(t.Context(), one, here)
	if err != nil {
		t.Fatal(err)
	}

	var logged bytes.Buffer
	trail := &recording{}
	raced := NewService(&racing{serving: db, marketplace: one}, NewHasher(cheap, 1), breachedNone, trail,
		slog.New(slog.NewTextHandler(&logged, nil)))
	err = raced.ChangePassword(t.Context(), visit(one), session, "correct horse battery staple", "a brand new passphrase")
	if !errors.Is(err, ErrCredentials) {
		t.Fatalf("ChangePassword = %v, want ErrCredentials", err)
	}
	if _, err := s.Authenticate(t.Context(), one, there); err != nil {
		t.Fatalf("a refused change ended the other session: %v", err)
	}
	if len(trail.entries) != 0 {
		t.Errorf("a refused change was audited: %v", trail.actions())
	}
	if !strings.Contains(logged.String(), "the password changed while it was being verified") {
		t.Errorf("the refusal left no trace in the log: %q", logged.String())
	}
	if strings.Contains(logged.String(), "correct horse") || strings.Contains(logged.String(), "brand new") {
		t.Errorf("the log carries a password: %q", logged.String())
	}
}
