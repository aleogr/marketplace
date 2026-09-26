//go:build integration

package identity

import (
	"bytes"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestChangingThePasswordEndsEverySessionAndRenewsTheOneThatAsked(t *testing.T) {
	s, db, one, _, trail := service(t)
	confirmed(t, s, db, one, "r@example.test")
	here := signIn(t, s, one, "r@example.test", "correct horse battery staple")
	there := signIn(t, s, one, "r@example.test", "correct horse battery staple")
	session, err := s.Authenticate(t.Context(), one, here)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := s.ChangePassword(t.Context(), visit(one), session, "wrong current password", "a brand new passphrase"); !errors.Is(err, ErrCredentials) {
		t.Fatalf("wrong current: %v, want ErrCredentials", err)
	}
	if _, err := s.ChangePassword(t.Context(), visit(one), session, "correct horse battery staple", "password1234"); !errors.Is(err, ErrPasswordBreached) {
		t.Fatalf("breached new: %v, want ErrPasswordBreached", err)
	}
	if slices.Contains(trail.actions(), "identity.password_changed") {
		t.Fatalf("a refused change was audited as done: %v", trail.actions())
	}

	renewed, err := s.ChangePassword(t.Context(), visit(one), session, "correct horse battery staple", "a brand new passphrase")
	if err != nil {
		t.Fatal(err)
	}
	// The browser that changed the password stays signed in, on a new token:
	// a stolen copy of its old cookie ends with the change (OWASP Session
	// Management).
	if renewed == "" || renewed == here {
		t.Fatalf("ChangePassword returned %q, want a new token", renewed)
	}
	if again, err := s.Authenticate(t.Context(), one, renewed); err != nil || again.Account.ID != session.Account.ID || again.ID == session.ID {
		t.Fatalf("the new session = %+v, %v; want a new session of the same account", again, err)
	}
	if _, err := s.Authenticate(t.Context(), one, here); !errors.Is(err, ErrSessionInvalid) {
		t.Fatalf("the old token of the session that changed the password survived: %v", err)
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

	for name, token := range map[string]string{"old": here, "other": there} {
		var reason string
		if err := db.InTxFor(t.Context(), one, func(tx pgx.Tx) error {
			return tx.QueryRow(t.Context(), `SELECT revoked_reason FROM session WHERE token_hash = $1`,
				HashToken(token)).Scan(&reason)
		}); err != nil {
			t.Fatal(err)
		}
		if reason != "password_changed" {
			t.Errorf("the %s session's revoked_reason = %q, want password_changed", name, reason)
		}
	}
	var agent, ip string
	if err := db.InTxFor(t.Context(), one, func(tx pgx.Tx) error {
		return tx.QueryRow(t.Context(), `SELECT user_agent, ip FROM session WHERE token_hash = $1`,
			HashToken(renewed)).Scan(&agent, &ip)
	}); err != nil {
		t.Fatal(err)
	}
	if agent != visit(one).UserAgent || ip != visit(one).IP {
		t.Errorf("the new session was opened for %s, %s; want the visit's %s, %s", agent, ip, visit(one).UserAgent, visit(one).IP)
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

	if _, err := s.ChangePassword(t.Context(), visit(one), session, "correct horse battery staple", "a brand new passphrase"); err != nil {
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
	_, err = raced.ChangePassword(t.Context(), visit(one), session, "correct horse battery staple", "a brand new passphrase")
	if !errors.Is(err, ErrCredentials) {
		t.Fatalf("ChangePassword = %v, want ErrCredentials", err)
	}
	for _, token := range []string{here, there} {
		if _, err := s.Authenticate(t.Context(), one, token); err != nil {
			t.Fatalf("a refused change ended a session: %v", err)
		}
	}
	if len(trail.entries) != 0 {
		t.Errorf("a refused change was audited: %v", trail.actions())
	}
	var secret string
	if err := db.InTxFor(t.Context(), one, func(tx pgx.Tx) error {
		var err error
		secret, err = passwordOf(t.Context(), tx, session.Account.ID)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if secret != "$argon2id$another" {
		t.Errorf("the credential = %q, want the racer's value: the refused change overwrote it", secret)
	}
	if !strings.Contains(logged.String(), "the password changed while it was being verified") {
		t.Errorf("the refusal left no trace in the log: %q", logged.String())
	}
	if strings.Contains(logged.String(), "correct horse") || strings.Contains(logged.String(), "brand new") {
		t.Errorf("the log carries a password: %q", logged.String())
	}
}

// A sign-in's second step opened with the old password dies with it: a right
// code on that challenge, after the password changed in another session, is
// refused as a challenge that no longer exists (D3, D2).
func TestAPasswordChangeEndsTheSignInsTheOldPasswordOpened(t *testing.T) {
	s, db, one, _, _ := service(t)
	sealed(t, s)
	session, token, enrolment := withAppSignedIn(t, s, db, one, "r@example.test")
	opened := challenged(t, s, one, "r@example.test")

	stepped := stepUp(t, s, one, token, session, ActionPassword, enrolment)
	if _, err := s.ChangePassword(t.Context(), visit(one), stepped, "correct horse battery staple", "a brand new passphrase"); err != nil {
		t.Fatalf("ChangePassword = %v", err)
	}
	s.now = func() time.Time { return time.Now().Add(time.Minute) }
	if _, err := s.CompleteSignIn(t.Context(), visit(one), opened,
		Answer{Method: MethodApp, Code: nextAppCode(t, s, enrolment)}); !errors.Is(err, ErrChallengeInvalid) {
		t.Fatalf("the old password's challenge after the change = %v, want ErrChallengeInvalid", err)
	}
}

// A sign-in whose password was checked before a change, and whose challenge
// is written while the change waits for the password's row, is ended too:
// the change deletes the challenges after it holds the credential, so it
// sees every challenge the old password opened.
func TestASignInOpenedWhileThePasswordChangesIsEnded(t *testing.T) {
	s, db, one, _, _ := service(t)
	sealed(t, s)
	session, token, enrolment := withAppSignedIn(t, s, db, one, "r@example.test")
	stepped := stepUp(t, s, one, token, session, ActionPassword, enrolment)
	account := session.Account.ID
	opened, hash, err := NewToken()
	if err != nil {
		t.Fatal(err)
	}

	// The sign-in's transaction, held between locking the password it
	// verified and writing its challenge, as SignIn takes them.
	holding := make(chan string, 1)
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseA := func() { releaseOnce.Do(func() { close(release) }) }
	defer releaseA()
	doneA := make(chan error, 1)
	go func() {
		doneA <- db.InTxFor(t.Context(), one, func(tx pgx.Tx) error {
			var secret, xid string
			if err := tx.QueryRow(t.Context(), `
				SELECT secret, xid(pg_current_xact_id())::text FROM credential
				 WHERE account_id = $1 AND kind = 'password' FOR UPDATE`, account).Scan(&secret, &xid); err != nil {
				return err
			}
			holding <- xid
			<-release
			return insertChallenge(t.Context(), tx, one, account, hash, "", "", time.Now())
		})
	}()
	var xid string
	select {
	case xid = <-holding:
	case err := <-doneA:
		t.Fatalf("the sign-in = %v", err)
	}

	doneB := make(chan error, 1)
	go func() {
		_, err := s.ChangePassword(t.Context(), visit(one), stepped, "correct horse battery staple", "a brand new passphrase")
		doneB <- err
	}()
	waitsFor(t, db, one, xid, doneB)
	releaseA()
	if err := <-doneA; err != nil {
		t.Fatal(err)
	}
	if err := <-doneB; err != nil {
		t.Fatalf("ChangePassword = %v", err)
	}
	s.now = func() time.Time { return time.Now().Add(time.Minute) }
	if _, err := s.CompleteSignIn(t.Context(), visit(one), opened,
		Answer{Method: MethodApp, Code: nextAppCode(t, s, enrolment)}); !errors.Is(err, ErrChallengeInvalid) {
		t.Fatalf("the challenge written during the change = %v, want ErrChallengeInvalid", err)
	}
}

// A password change leaves the step-ups of other sessions alone: they are
// not sign-ins, and the session that would answer one was ended with the
// rest.
func TestAPasswordChangeKeepsItsOwnStepUps(t *testing.T) {
	s, db, one, _, _ := service(t)
	sealed(t, s)
	session, token, enrolment := withAppSignedIn(t, s, db, one, "r@example.test")
	stepped := stepUp(t, s, one, token, session, ActionPassword, enrolment)
	pending, err := s.BeginStepUp(t.Context(), visit(one), stepped, ActionFactors)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ChangePassword(t.Context(), visit(one), stepped, "correct horse battery staple", "a brand new passphrase"); err != nil {
		t.Fatalf("ChangePassword = %v", err)
	}
	var left int
	if err := db.InTxFor(t.Context(), one, func(tx pgx.Tx) error {
		return tx.QueryRow(t.Context(), `SELECT count(*) FROM sign_in_challenge WHERE token_hash = $1`, HashToken(pending)).Scan(&left)
	}); err != nil {
		t.Fatal(err)
	}
	if left != 1 {
		t.Fatalf("the step-up's challenge rows after the change = %d, want 1: only sign-ins are ended", left)
	}
}
