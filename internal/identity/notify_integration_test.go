//go:build integration

package identity

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
)

// mailed is the variables of the last mail of template requested in a
// marketplace, and its address.
func mailed(t *testing.T, db serving, marketplace, template string) (string, map[string]string) {
	t.Helper()
	var to string
	variables := map[string]string{}
	if err := db.InTxFor(t.Context(), marketplace, func(tx pgx.Tx) error {
		return tx.QueryRow(t.Context(), `
			SELECT payload->>'To', payload->'Variables' FROM outbox_event
			 WHERE kind = 'email.send' AND payload->>'Template' = $1
			 ORDER BY created_at DESC, id DESC LIMIT 1`, template).Scan(&to, &variables)
	}); err != nil {
		t.Fatalf("no %s mail was requested: %v", template, err)
	}
	return to, variables
}

// Adding a method and removing one each tell the owner, naming the method.
func TestChangingTheFactorsTellsTheOwner(t *testing.T) {
	s, db, one, _, _ := service(t)
	sealed(t, s)
	session := signedIn(t, s, db, one, "r@example.test")
	enrolApp(t, s, visit(one), session, "phone")
	if to, variables := mailed(t, db, one, "second-factor-added"); to != "r@example.test" ||
		variables["Method"] != "totp" || variables["Name"] != "Reader" {
		t.Fatalf("second-factor-added to %s with %v", to, variables)
	}

	security, err := s.Security(t.Context(), visit(one), session)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.RemoveFactor(t.Context(), visit(one), session, security.Factors[0].ID); err != nil {
		t.Fatal(err)
	}
	if to, variables := mailed(t, db, one, "second-factor-removed"); to != "r@example.test" || variables["Method"] != "totp" {
		t.Fatalf("second-factor-removed to %s with %v", to, variables)
	}
}

// A recovery code used to sign in tells the owner how many are left.
func TestARecoveryCodeUsedTellsTheOwner(t *testing.T) {
	s, db, one, _, _ := service(t)
	sealed(t, s)
	_, codes := withApp(t, s, db, one, "r@example.test")
	if _, err := s.CompleteSignIn(t.Context(), visit(one), challenged(t, s, one, "r@example.test"),
		Answer{Method: MethodRecovery, Code: codes[0]}); err != nil {
		t.Fatal(err)
	}
	if to, variables := mailed(t, db, one, "recovery-code-used"); to != "r@example.test" || variables["Left"] != "9" {
		t.Fatalf("recovery-code-used to %s with %v", to, variables)
	}
}

// noticeCount is how many mails of template have been requested in a
// marketplace so far.
func noticeCount(t *testing.T, db serving, marketplace, template string) int {
	t.Helper()
	var count int
	if err := db.InTxFor(t.Context(), marketplace, func(tx pgx.Tx) error {
		return tx.QueryRow(t.Context(), `
			SELECT count(*) FROM outbox_event
			 WHERE kind = 'email.send' AND payload->>'Template' = $1`, template).Scan(&count)
	}); err != nil {
		t.Fatalf("counting %s mail: %v", template, err)
	}
	return count
}

// A refused change is not a change, so it must tell the owner nothing: only
// a wrong app code, a wrong e-mail code, a wrong recovery code and a key
// whose credential id is already added are exercised here, each checked
// against the outbox rather than the code, and a success at the end proves
// the count is not simply stuck at zero.
func TestARefusalSendsNoNotice(t *testing.T) {
	s, db, one, two, _ := service(t)
	sealed(t, s)

	// A wrong app code, on a fresh account with no factor yet: nothing is
	// added, so nothing is requested.
	session := signedIn(t, s, db, one, "r@example.test")
	v := visit(one)
	if _, err := s.BeginApp(t.Context(), v, session); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ConfirmApp(t.Context(), v, session, "phone", "000000"); !errors.Is(err, ErrCodeWrong) {
		t.Fatalf("a wrong app code = %v, want ErrCodeWrong", err)
	}
	if got := noticeCount(t, db, one, "second-factor-added"); got != 0 {
		t.Fatalf("second-factor-added after a wrong app code = %d, want 0", got)
	}

	// A wrong e-mail code: still nothing added, still nothing requested.
	if err := s.BeginEmail(t.Context(), v, session); err != nil {
		t.Fatal(err)
	}
	code := lastCode(t, db, one)
	if err := s.ConfirmEmail(t.Context(), v, session, otherCode(code)); !errors.Is(err, ErrCodeWrong) {
		t.Fatalf("a wrong e-mail code = %v, want ErrCodeWrong", err)
	}
	if got := noticeCount(t, db, one, "second-factor-added"); got != 0 {
		t.Fatalf("second-factor-added after a wrong e-mail code = %d, want 0", got)
	}

	// A wrong recovery code, on a different account with a real app, so
	// signing in is possible: no recovery code is spent, so no notice names
	// how many are left.
	holder := signedIn(t, s, db, two, "h@example.test")
	enrolApp(t, s, visit(two), holder, "phone")
	if _, err := s.CompleteSignIn(t.Context(), visit(two), challenged(t, s, two, "h@example.test"),
		Answer{Method: MethodRecovery, Code: "0000-0000-0000"}); !errors.Is(err, ErrCodeWrong) {
		t.Fatalf("a wrong recovery code = %v, want ErrCodeWrong", err)
	}
	if got := noticeCount(t, db, two, "recovery-code-used"); got != 0 {
		t.Fatalf("recovery-code-used after a wrong recovery code = %d, want 0", got)
	}

	// A key whose credential id is already added is refused (soft key,
	// following TestAKeyAlreadyAddedIsRefused's approach): the attempt adds
	// no factor, so it requests no notice either. The credential's owner
	// added it moments ago, which did request one, so the count checked here
	// is the one just before this attempt, not zero.
	_, key, _ := withKey(t, s, db, two, "k@example.test")
	another := signedIn(t, s, db, two, "m@example.test")
	before := noticeCount(t, db, two, "second-factor-added")
	options, err := s.BeginKey(t.Context(), visit(two), another)
	if err != nil {
		t.Fatal(err)
	}
	copied := newSoftKey(t, visit(two).BaseURL)
	copied.id = key.id
	if _, err := s.ConfirmKey(t.Context(), visit(two), another, "", copied.register(options)); !errors.Is(err, ErrKeyRefused) {
		t.Fatalf("a key already added = %v, want ErrKeyRefused", err)
	}
	if got := noticeCount(t, db, two, "second-factor-added"); got != before {
		t.Fatalf("second-factor-added after a refused key = %d, want %d", got, before)
	}

	// The positive control: a real success, right after all those refusals,
	// does request its notice. If the test above were vacuous — passing
	// however notify is wired — this catches it.
	if err := s.ConfirmEmail(t.Context(), v, session, code); err != nil {
		t.Fatal(err)
	}
	if got := noticeCount(t, db, one, "second-factor-added"); got != 1 {
		t.Fatalf("second-factor-added after a successful e-mail enrolment = %d, want 1", got)
	}
}
