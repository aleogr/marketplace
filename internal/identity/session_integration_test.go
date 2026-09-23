//go:build integration

package identity

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/aleogr/marketplace/internal/identity/breached"
	"github.com/aleogr/marketplace/internal/platform/audit"
)

// breachedNone calls no password breached.
var breachedNone = breached.Fake{}

// entry returns the first entry recorded for action.
func (r *recording) entry(t *testing.T, action string) audit.Entry {
	t.Helper()
	for _, e := range r.entries {
		if e.Action == action {
			return e
		}
	}
	t.Fatalf("no %s among the audited actions %v", action, r.actions())
	return audit.Entry{}
}

// confirmed signs up and confirms an account with the given address.
func confirmed(t *testing.T, s *Service, db serving, marketplace, email string) {
	t.Helper()
	if err := s.SignUp(t.Context(), visit(marketplace), "Reader", email, "correct horse battery staple"); err != nil {
		t.Fatal(err)
	}
	mails := outbox(t, db, marketplace)
	last := mails[len(mails)-1]
	if last.template != "verify-email" {
		t.Fatalf("the last mail is %v, want the verification of %s", last, email)
	}
	if err := s.Verify(t.Context(), visit(marketplace), tokenOf(t, last.link)); err != nil {
		t.Fatal(err)
	}
}

// signIn signs in and fails the test if that is refused.
func signIn(t *testing.T, s *Service, marketplace, email, password string) string {
	t.Helper()
	token, err := s.SignIn(t.Context(), visit(marketplace), email, password)
	if err != nil {
		t.Fatalf("SignIn(%s) = %v", email, err)
	}
	return token
}

func TestSignInOutAndRevocation(t *testing.T) {
	s, db, one, _, trail := service(t)
	confirmed(t, s, db, one, "r@example.test")

	token := signIn(t, s, one, "R@Example.Test", "correct horse battery staple")
	session, err := s.Authenticate(t.Context(), one, token)
	if err != nil || session.Account.Email != "r@example.test" {
		t.Fatalf("Authenticate = %+v, %v", session, err)
	}

	// Each sign-in opens a session of its own; none is ever reused.
	other := signIn(t, s, one, "r@example.test", "correct horse battery staple")
	if other == token {
		t.Fatal("a second sign-in returned the first session's token")
	}
	second, err := s.Authenticate(t.Context(), one, other)
	if err != nil || second.ID == session.ID {
		t.Fatalf("the second sign-in's session = %+v, %v; want a new one", second, err)
	}

	if err := s.SignOut(t.Context(), visit(one), token); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Authenticate(t.Context(), one, token); !errors.Is(err, ErrSessionInvalid) {
		t.Fatalf("Authenticate after sign-out = %v, want ErrSessionInvalid", err)
	}
	if _, err := s.Authenticate(t.Context(), one, other); err != nil {
		t.Fatalf("signing one session out ended the other: %v", err)
	}
	// Signing out a session that is already over is not an error, and records
	// nothing more.
	if err := s.SignOut(t.Context(), visit(one), token); err != nil {
		t.Fatalf("a second SignOut = %v, want nil", err)
	}
	want := []string{"identity.signup", "identity.email_verified", "identity.signin", "identity.signin", "identity.signout"}
	if got := trail.actions(); !slices.Equal(got, want) {
		t.Fatalf("audited %v, want %v", got, want)
	}
	signin := trail.entry(t, "identity.signin")
	if signin.Actor.Kind != audit.Buyer || signin.Actor.ID != session.Account.ID || signin.Subject.Person != session.Account.ID {
		t.Fatalf("signin recorded as %+v", signin)
	}
}

func TestWrongPasswordAndUnknownAddressLookAlike(t *testing.T) {
	s, db, one, _, trail := service(t)
	confirmed(t, s, db, one, "r@example.test")
	_, wrong := s.SignIn(t.Context(), visit(one), "r@example.test", "not the password at all")
	_, unknown := s.SignIn(t.Context(), visit(one), "nobody@example.test", "not the password at all")
	_, malformed := s.SignIn(t.Context(), visit(one), "not an address", "not the password at all")
	if !errors.Is(wrong, ErrCredentials) || !errors.Is(unknown, ErrCredentials) || !errors.Is(malformed, ErrCredentials) {
		t.Fatalf("wrong = %v, unknown = %v, malformed = %v; want ErrCredentials for all", wrong, unknown, malformed)
	}

	// The failure against an existing account is audited, with the platform as
	// the actor and nobody as the person it is about: the address it came
	// from is not sealed under the account owner's key.
	failed := trail.entry(t, "identity.signin_failed")
	if failed.Actor.Kind != audit.System || failed.Actor.ID != "" || failed.Subject.Person != "" ||
		failed.Subject.Kind != "account" || failed.Subject.ID == "" || failed.From != "203.0.113.7" {
		t.Fatalf("signin_failed recorded as %+v", failed)
	}
	// The ones against no account are only logged: there is no account to
	// record them against.
	var failures int
	for _, action := range trail.actions() {
		if action == "identity.signin_failed" {
			failures++
		}
	}
	if failures != 1 {
		t.Fatalf("audited %v, want one identity.signin_failed", trail.actions())
	}
}

func TestAnUnconfirmedAccountCannotSignIn(t *testing.T) {
	s, _, one, _, _ := service(t)
	if err := s.SignUp(t.Context(), visit(one), "R", "r@example.test", "correct horse battery staple"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SignIn(t.Context(), visit(one), "r@example.test", "correct horse battery staple"); !errors.Is(err, ErrUnverified) {
		t.Fatalf("SignIn(unconfirmed) = %v, want ErrUnverified", err)
	}
	// Only the right password learns that the address awaits confirmation.
	if _, err := s.SignIn(t.Context(), visit(one), "r@example.test", "not the password at all"); !errors.Is(err, ErrCredentials) {
		t.Fatalf("SignIn(unconfirmed, wrong password) = %v, want ErrCredentials", err)
	}
}

func TestIdleAndExpiredSessionsAreRefused(t *testing.T) {
	s, db, one, _, _ := service(t)
	confirmed(t, s, db, one, "r@example.test")
	idle := signIn(t, s, one, "r@example.test", "correct horse battery staple")
	old := signIn(t, s, one, "r@example.test", "correct horse battery staple")
	if err := db.InTxFor(t.Context(), one, func(tx pgx.Tx) error {
		if _, err := tx.Exec(t.Context(),
			`UPDATE session SET last_seen_at = now() - interval '31 days' WHERE token_hash = $1`, HashToken(idle)); err != nil {
			return err
		}
		_, err := tx.Exec(t.Context(),
			`UPDATE session SET created_at = now() - interval '91 days' WHERE token_hash = $1`, HashToken(old))
		return err
	}); err != nil {
		t.Fatal(err)
	}
	for name, token := range map[string]string{"idle": idle, "expired": old} {
		if _, err := s.Authenticate(t.Context(), one, token); !errors.Is(err, ErrSessionInvalid) {
			t.Errorf("%s session: %v, want ErrSessionInvalid", name, err)
		}
	}
	if _, err := s.Authenticate(t.Context(), one, "never issued"); !errors.Is(err, ErrSessionInvalid) {
		t.Errorf("unknown session: %v, want ErrSessionInvalid", err)
	}
}

func TestLastSeenIsWrittenAtMostOnceAnHour(t *testing.T) {
	s, db, one, _, _ := service(t)
	confirmed(t, s, db, one, "r@example.test")
	token := signIn(t, s, one, "r@example.test", "correct horse battery staple")

	// lastSeen sets last_seen_at back by ago and returns what Authenticate
	// leaves it at.
	lastSeen := func(ago string) (moved bool) {
		t.Helper()
		if err := db.InTxFor(t.Context(), one, func(tx pgx.Tx) error {
			_, err := tx.Exec(t.Context(),
				`UPDATE session SET last_seen_at = now() - $2::interval WHERE token_hash = $1`, HashToken(token), ago)
			return err
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Authenticate(t.Context(), one, token); err != nil {
			t.Fatal(err)
		}
		if err := db.InTxFor(t.Context(), one, func(tx pgx.Tx) error {
			return tx.QueryRow(t.Context(),
				`SELECT last_seen_at > now() - interval '1 minute' FROM session WHERE token_hash = $1`,
				HashToken(token)).Scan(&moved)
		}); err != nil {
			t.Fatal(err)
		}
		return moved
	}
	if lastSeen("10 minutes") {
		t.Error("a session seen ten minutes ago was written again")
	}
	if !lastSeen("2 hours") {
		t.Error("a session seen two hours ago was not brought up to date")
	}
}

func TestASessionIsInvisibleFromAnotherMarketplace(t *testing.T) {
	s, db, one, two, _ := service(t)
	confirmed(t, s, db, one, "r@example.test")
	token := signIn(t, s, one, "r@example.test", "correct horse battery staple")
	if _, err := s.Authenticate(t.Context(), one, token); err != nil {
		t.Fatalf("the session does not work in its own marketplace: %v", err)
	}
	if _, err := s.Authenticate(t.Context(), two, token); !errors.Is(err, ErrSessionInvalid) {
		t.Fatalf("a session of one marketplace authenticated in the other: %v", err)
	}
	// Nor can the other marketplace sign it out.
	if err := s.SignOut(t.Context(), visit(two), token); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Authenticate(t.Context(), one, token); err != nil {
		t.Fatalf("another marketplace signed the session out: %v", err)
	}
}

func TestAStaleHashIsReplacedAtSignIn(t *testing.T) {
	db, one, _ := twoMarketplaces(t)
	old := NewService(db, NewHasher(cheap, 1), breachedNone, &recording{}, quiet)
	confirmed(t, old, db, one, "r@example.test")
	newer := NewService(db, NewHasher(Params{Memory: 128, Time: 1, Threads: 1, KeyLen: 32, SaltLen: 16}, 1),
		breachedNone, &recording{}, quiet)
	signIn(t, newer, one, "r@example.test", "correct horse battery staple")

	var secret string
	if err := db.InTxFor(t.Context(), one, func(tx pgx.Tx) error {
		return tx.QueryRow(t.Context(), "SELECT secret FROM credential").Scan(&secret)
	}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(secret, "m=128,") {
		t.Fatalf("the hash was not refreshed: %s", secret)
	}
	// And the refreshed hash still opens the account.
	signIn(t, newer, one, "r@example.test", "correct horse battery staple")
}

// The second transaction of a sign-in writes only if the password it verified
// is still the account's.
func TestPasswordStillNoticesAChange(t *testing.T) {
	s, db, one, _, _ := service(t)
	confirmed(t, s, db, one, "r@example.test")
	if err := db.InTxFor(t.Context(), one, func(tx pgx.Tx) error {
		account, err := accountByEmail(t.Context(), tx, "r@example.test")
		if err != nil {
			return err
		}
		verified, err := passwordOf(t.Context(), tx, account.ID)
		if err != nil {
			return err
		}
		if still, err := passwordStill(t.Context(), tx, account.ID, verified); err != nil || !still {
			t.Errorf("passwordStill(unchanged) = %v, %v; want true", still, err)
		}
		if err := setPassword(t.Context(), tx, one, account.ID, "$argon2id$another"); err != nil {
			return err
		}
		if still, err := passwordStill(t.Context(), tx, account.ID, verified); err != nil || still {
			t.Errorf("passwordStill(changed) = %v, %v; want false", still, err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
