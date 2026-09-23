//go:build integration

package identity

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"slices"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/aleogr/marketplace/internal/identity/breached"
	"github.com/aleogr/marketplace/internal/platform/audit"
)

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

// recording is an Auditor that remembers the entries it was given.
type recording struct{ entries []audit.Entry }

func (r *recording) Append(_ context.Context, _ pgx.Tx, e audit.Entry) error {
	r.entries = append(r.entries, e)
	return nil
}

// actions are the recorded actions, in order.
func (r *recording) actions() []string {
	var out []string
	for _, e := range r.entries {
		out = append(out, e.Action)
	}
	return out
}

func service(t *testing.T) (*Service, serving, string, string, *recording) {
	db, one, two := twoMarketplaces(t)
	trail := &recording{}
	s := NewService(db, NewHasher(cheap, 2), breached.Fake{Known: breached.Common}, trail, quiet)
	return s, db, one, two, trail
}

func visit(marketplace string) Visit {
	return Visit{Marketplace: marketplace, MarketplaceName: "One", Language: "pt-BR",
		IP: "203.0.113.7", UserAgent: "test", BaseURL: "https://one.test"}
}

// sent is one message requested for delivery.
type sent struct{ template, link string }

// outbox returns the messages requested for delivery in a marketplace, oldest
// first.
func outbox(t *testing.T, db serving, marketplace string) []sent {
	t.Helper()
	var got []sent
	if err := db.InTxFor(t.Context(), marketplace, func(tx pgx.Tx) error {
		rows, err := tx.Query(t.Context(),
			`SELECT payload->>'Template', coalesce(payload->'Variables'->>'Link', '')
			   FROM outbox_event WHERE kind = 'email.send' ORDER BY created_at, id`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var one sent
			if err := rows.Scan(&one.template, &one.link); err != nil {
				return err
			}
			got = append(got, one)
		}
		return rows.Err()
	}); err != nil {
		t.Fatalf("cannot read the outbox: %v", err)
	}
	return got
}

// tokenOf is the token a verification link carries.
func tokenOf(t *testing.T, link string) string {
	t.Helper()
	_, token, found := strings.Cut(link, "token=")
	if !found {
		t.Fatalf("%q carries no token", link)
	}
	return token
}

func TestSignUpAndVerify(t *testing.T) {
	s, db, one, _, trail := service(t)
	if err := s.SignUp(t.Context(), visit(one), "Reader", "Reader@Example.Test", "correct horse battery staple"); err != nil {
		t.Fatal(err)
	}
	mails := outbox(t, db, one)
	if len(mails) != 1 || mails[0].template != "verify-email" {
		t.Fatalf("outbox = %v, want one verify-email", mails)
	}
	if !strings.HasPrefix(mails[0].link, "https://one.test/pt-BR/verify?token=") {
		t.Fatalf("link %q is not this marketplace's verification page", mails[0].link)
	}
	token := tokenOf(t, mails[0].link)

	if err := s.Verify(t.Context(), visit(one), token); err != nil {
		t.Fatalf("Verify = %v", err)
	}
	if err := s.Verify(t.Context(), visit(one), token); !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("second Verify = %v, want ErrTokenInvalid: a token is single-use", err)
	}
	if got := trail.actions(); !slices.Equal(got, []string{"identity.signup", "identity.email_verified"}) {
		t.Fatalf("audited %v", got)
	}
}

func TestSignUpWithATakenAddressSendsAccountExistsAndSaysNothing(t *testing.T) {
	s, db, one, _, _ := service(t)
	if err := s.SignUp(t.Context(), visit(one), "Reader", "reader@example.test", "correct horse battery staple"); err != nil {
		t.Fatal(err)
	}
	if err := s.SignUp(t.Context(), visit(one), "Someone", "READER@example.test", "another long passphrase"); err != nil {
		t.Fatalf("second SignUp = %v, want nil: the answer must not reveal the account", err)
	}
	mails := outbox(t, db, one)
	if len(mails) != 2 || mails[0].template != "verify-email" || mails[1].template != "account-exists" {
		t.Fatalf("outbox = %v, want verify-email then account-exists", mails)
	}
}

func TestSignUpRefusesABreachedOrShortPassword(t *testing.T) {
	s, db, one, _, _ := service(t)
	if err := s.SignUp(t.Context(), visit(one), "R", "a@example.test", "password1234"); !errors.Is(err, ErrPasswordBreached) {
		t.Fatalf("breached: %v", err)
	}
	if err := s.SignUp(t.Context(), visit(one), "R", "a@example.test", "short"); !errors.Is(err, ErrPasswordShort) {
		t.Fatalf("short: %v", err)
	}
	if mails := outbox(t, db, one); len(mails) != 0 {
		t.Fatalf("a refused sign-up sent %v", mails)
	}
}

func TestAnUnreachableBreachServiceDoesNotBlockSignUp(t *testing.T) {
	db, one, _ := twoMarketplaces(t)
	s := NewService(db, NewHasher(cheap, 1), breached.Fake{Err: errors.New("down")}, &recording{}, quiet)
	if err := s.SignUp(t.Context(), visit(one), "R", "r@example.test", "correct horse battery staple"); err != nil {
		t.Fatalf("SignUp = %v, want nil (fail open, spec D4)", err)
	}
}

func TestResendReissuesOnlyToAnUnconfirmedAddress(t *testing.T) {
	s, db, one, _, _ := service(t)

	// An unconfirmed account.
	if err := s.SignUp(t.Context(), visit(one), "Reader", "reader@example.test", "correct horse battery staple"); err != nil {
		t.Fatal(err)
	}
	firstToken := tokenOf(t, outbox(t, db, one)[0].link)

	// A confirmed account, in the same marketplace.
	if err := s.SignUp(t.Context(), visit(one), "Verified", "verified@example.test", "correct horse battery staple"); err != nil {
		t.Fatal(err)
	}
	confirmedToken := tokenOf(t, outbox(t, db, one)[1].link)
	if err := s.Verify(t.Context(), visit(one), confirmedToken); err != nil {
		t.Fatal(err)
	}

	if err := s.Resend(t.Context(), visit(one), "unknown@example.test"); err != nil {
		t.Fatalf("Resend(unknown) = %v, want nil", err)
	}
	if err := s.Resend(t.Context(), visit(one), "VERIFIED@example.test"); err != nil {
		t.Fatalf("Resend(confirmed) = %v, want nil", err)
	}
	if err := s.Resend(t.Context(), visit(one), "READER@example.test"); err != nil {
		t.Fatalf("Resend(unconfirmed) = %v, want nil", err)
	}

	// Exactly one message more than the two sign-ups sent: the unconfirmed
	// address's, and none for the unknown or already-confirmed ones.
	mails := outbox(t, db, one)
	if len(mails) != 3 || mails[2].template != "verify-email" {
		t.Fatalf("outbox = %v, want exactly one more verify-email", mails)
	}

	newToken := tokenOf(t, mails[2].link)
	if newToken == firstToken {
		t.Fatalf("Resend reissued the sign-up's own token instead of a new one")
	}
	if err := s.Verify(t.Context(), visit(one), newToken); err != nil {
		t.Fatalf("the reissued token does not verify: %v", err)
	}
}

func TestAnExpiredTokenIsRefused(t *testing.T) {
	s, db, one, _, _ := service(t)
	if err := s.SignUp(t.Context(), visit(one), "R", "r@example.test", "correct horse battery staple"); err != nil {
		t.Fatal(err)
	}
	token := tokenOf(t, outbox(t, db, one)[0].link)
	if err := db.InTxFor(t.Context(), one, func(tx pgx.Tx) error {
		_, err := tx.Exec(t.Context(), "UPDATE email_verification SET expires_at = now() - interval '1 second'")
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.Verify(t.Context(), visit(one), token); !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("Verify(expired) = %v, want ErrTokenInvalid", err)
	}
}
