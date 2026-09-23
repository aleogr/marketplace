//go:build integration

package identity

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"maps"
	"slices"
	"strings"
	"testing"
	"time"

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

// A session's clock is the service's, like every check made against it: a
// database clock that drifts from the process's would make a session idle or
// expired early or late.
func TestASessionStartsAtTheServicesClock(t *testing.T) {
	s, db, one, _, _ := service(t)
	confirmed(t, s, db, one, "r@example.test")
	at := time.Now().UTC().Add(-3 * time.Hour).Truncate(time.Microsecond)
	s.now = func() time.Time { return at }
	token := signIn(t, s, one, "r@example.test", "correct horse battery staple")

	var created, seen time.Time
	if err := db.InTxFor(t.Context(), one, func(tx pgx.Tx) error {
		return tx.QueryRow(t.Context(), `SELECT created_at, last_seen_at FROM session WHERE token_hash = $1`,
			HashToken(token)).Scan(&created, &seen)
	}); err != nil {
		t.Fatal(err)
	}
	if !created.Equal(at) || !seen.Equal(at) {
		t.Errorf("created_at %v, last_seen_at %v; want both %v", created, seen, at)
	}
}

// racing is a Transactor that changes the account's password just before the
// second transaction opens: what another sign-in's rehash, or a password
// change, does when it lands between a sign-in's read and its write.
type racing struct {
	serving
	marketplace string
	calls       int
}

func (r *racing) InTxFor(ctx context.Context, marketplaceID string, fn func(pgx.Tx) error) error {
	r.calls++
	if r.calls == 2 {
		if err := r.serving.InTxFor(ctx, r.marketplace, func(tx pgx.Tx) error {
			_, err := tx.Exec(ctx, `UPDATE credential SET secret = '$argon2id$another'`)
			return err
		}); err != nil {
			return err
		}
	}
	return r.serving.InTxFor(ctx, marketplaceID, fn)
}

// A correct password whose hash changed while it was being verified is
// refused, and the refusal leaves a line in the log, without the address.
func TestAPasswordChangedMidSignInIsRefusedAndLogged(t *testing.T) {
	s, db, one, _, _ := service(t)
	confirmed(t, s, db, one, "r@example.test")

	var logged bytes.Buffer
	raced := NewService(&racing{serving: db, marketplace: one}, NewHasher(cheap, 1), breachedNone, &recording{},
		slog.New(slog.NewTextHandler(&logged, nil)))
	_, err := raced.SignIn(t.Context(), visit(one), "r@example.test", "correct horse battery staple")
	if !errors.Is(err, ErrCredentials) {
		t.Fatalf("SignIn = %v, want ErrCredentials", err)
	}
	if !strings.Contains(logged.String(), "the password changed while it was being verified") {
		t.Errorf("the refusal left no trace in the log: %q", logged.String())
	}
	if strings.Contains(logged.String(), "r@example.test") || strings.Contains(logged.String(), "correct horse") {
		t.Errorf("the log carries the address or the password: %q", logged.String())
	}
}

// A session replaced by a new sign-in in the same browser is revoked with
// its own reason, and recorded.
func TestSupersedeRevokesTheSession(t *testing.T) {
	s, db, one, _, trail := service(t)
	confirmed(t, s, db, one, "r@example.test")
	token := signIn(t, s, one, "r@example.test", "correct horse battery staple")

	if err := s.Supersede(t.Context(), visit(one), token); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Authenticate(t.Context(), one, token); !errors.Is(err, ErrSessionInvalid) {
		t.Fatalf("a superseded session still authenticates: %v", err)
	}
	var reason string
	if err := db.InTxFor(t.Context(), one, func(tx pgx.Tx) error {
		return tx.QueryRow(t.Context(), `SELECT revoked_reason FROM session WHERE token_hash = $1`,
			HashToken(token)).Scan(&reason)
	}); err != nil {
		t.Fatal(err)
	}
	if reason != "replaced" {
		t.Errorf("revoked_reason = %q, want replaced", reason)
	}
	if !slices.Contains(trail.actions(), "identity.session_replaced") {
		t.Errorf("the replacement was not audited: %v", trail.actions())
	}
	// A token that opens nothing is not an error.
	if err := s.Supersede(t.Context(), visit(one), token); err != nil {
		t.Errorf("superseding a revoked session = %v, want nil", err)
	}
}

// hashExists reports whether a session row with this token hash still
// exists, revoked or not.
func hashExists(t *testing.T, db serving, marketplace string, hash []byte) bool {
	t.Helper()
	var exists bool
	if err := db.InTxFor(t.Context(), marketplace, func(tx pgx.Tx) error {
		return tx.QueryRow(t.Context(), `SELECT EXISTS(SELECT 1 FROM session WHERE token_hash = $1)`,
			hash).Scan(&exists)
	}); err != nil {
		t.Fatal(err)
	}
	return exists
}

// seedSession inserts a session row with created_at, last_seen_at and
// revoked_at set directly, so a test can put a row on either side of the
// sweep's three conditions without waiting real time out.
func seedSession(t *testing.T, db serving, marketplace, account string, hash []byte, created, seen time.Time, revoked *time.Time) {
	t.Helper()
	if err := db.InTxFor(t.Context(), marketplace, func(tx pgx.Tx) error {
		if err := insertSession(t.Context(), tx, marketplace, account, hash, "203.0.113.7", "test", created); err != nil {
			return err
		}
		if _, err := tx.Exec(t.Context(),
			`UPDATE session SET last_seen_at = $2 WHERE token_hash = $1`, hash, seen); err != nil {
			return err
		}
		if revoked == nil {
			return nil
		}
		_, err := tx.Exec(t.Context(),
			`UPDATE session SET revoked_at = $2, revoked_reason = 'signout' WHERE token_hash = $1`, hash, *revoked)
		return err
	}); err != nil {
		t.Fatal(err)
	}
}

// Task 12b: sessions that can never authenticate again are removed. Each of
// the three conditions on its own is enough, and a live session and one
// revoked less than RevokedKept ago both survive.
func TestSweepRemovesSessionsPastEachCondition(t *testing.T) {
	db, one, _ := twoMarketplaces(t)
	var account string
	if err := db.InTxFor(t.Context(), one, func(tx pgx.Tx) error {
		var err error
		account, err = insertAccount(t.Context(), tx, one, "r@example.test", "r@example.test", "Reader")
		return err
	}); err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC()
	live := []byte("live-session")
	tooOld := []byte("too-old-session")
	tooIdle := []byte("too-idle-session")
	longRevoked := []byte("long-revoked-session")
	recentRevoked := []byte("recent-revoked-session")

	seedSession(t, db, one, account, live, now.Add(-time.Hour), now.Add(-time.Hour), nil)
	seedSession(t, db, one, account, tooOld, now.Add(-SessionLifetime-time.Hour), now.Add(-time.Hour), nil)
	seedSession(t, db, one, account, tooIdle, now.Add(-40*24*time.Hour), now.Add(-SessionIdle-time.Hour), nil)
	revokedLong := now.Add(-RevokedKept - time.Hour)
	seedSession(t, db, one, account, longRevoked, now.Add(-time.Hour), now.Add(-time.Hour), &revokedLong)
	revokedRecent := now.Add(-2 * 24 * time.Hour)
	seedSession(t, db, one, account, recentRevoked, now.Add(-time.Hour), now.Add(-time.Hour), &revokedRecent)

	var removed int64
	if err := db.InTxFor(t.Context(), one, func(tx pgx.Tx) error {
		var err error
		removed, err = sweepSessions(t.Context(), tx, now)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if removed != 3 {
		t.Errorf("sweepSessions removed %d rows, want 3", removed)
	}

	want := map[string]bool{"live": true, "tooOld": false, "tooIdle": false, "longRevoked": false, "recentRevoked": true}
	got := map[string]bool{
		"live":          hashExists(t, db, one, live),
		"tooOld":        hashExists(t, db, one, tooOld),
		"tooIdle":       hashExists(t, db, one, tooIdle),
		"longRevoked":   hashExists(t, db, one, longRevoked),
		"recentRevoked": hashExists(t, db, one, recentRevoked),
	}
	if !maps.Equal(got, want) {
		t.Fatalf("after the sweep, existence = %+v, want %+v", got, want)
	}
}

// The sweep of one marketplace never touches another's rows: row-level
// security is what scopes the DELETE, like every other query in this
// package.
func TestSweepNeverTouchesAnotherMarketplace(t *testing.T) {
	db, one, two := twoMarketplaces(t)
	var acctTwo string
	if err := db.InTxFor(t.Context(), two, func(tx pgx.Tx) error {
		var err error
		acctTwo, err = insertAccount(t.Context(), tx, two, "r@example.test", "r@example.test", "Reader")
		return err
	}); err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC()
	eligibleInTwo := []byte("eligible-in-two")
	seedSession(t, db, two, acctTwo, eligibleInTwo, now.Add(-SessionLifetime-time.Hour), now.Add(-time.Hour), nil)

	if err := db.InTxFor(t.Context(), one, func(tx pgx.Tx) error {
		_, err := sweepSessions(t.Context(), tx, now)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if !hashExists(t, db, two, eligibleInTwo) {
		t.Error("sweeping marketplace one removed a row that belongs to marketplace two")
	}
}

// The sweep SignIn triggers runs at most once an hour per process: a second
// sign-in within the hour does not remove a row that became eligible in
// between, and one after the hour does.
func TestSweepRunsAtMostOnceAnHour(t *testing.T) {
	s, db, one, _, _ := service(t)
	confirmed(t, s, db, one, "r@example.test")

	clock := time.Now().UTC()
	s.now = func() time.Time { return clock }

	// The process has swept nothing yet, so this sign-in is due; there is
	// nothing to remove, and it only records s.lastSweep.
	signIn(t, s, one, "r@example.test", "correct horse battery staple")

	// A session that becomes eligible for the sweep between two sign-ins.
	stale := signIn(t, s, one, "r@example.test", "correct horse battery staple")
	revokedAt := clock.Add(-RevokedKept - time.Hour)
	if err := db.InTxFor(t.Context(), one, func(tx pgx.Tx) error {
		_, err := tx.Exec(t.Context(),
			`UPDATE session SET revoked_at = $2, revoked_reason = 'signout' WHERE token_hash = $1`,
			HashToken(stale), revokedAt)
		return err
	}); err != nil {
		t.Fatal(err)
	}

	// Within the hour: the sweep does not run again, so the row survives.
	clock = clock.Add(10 * time.Minute)
	signIn(t, s, one, "r@example.test", "correct horse battery staple")
	if !hashExists(t, db, one, HashToken(stale)) {
		t.Fatal("a sign-in within the hour swept a session that became eligible in between")
	}

	// An hour after the first sweep: it runs again and removes the row.
	clock = clock.Add(55 * time.Minute)
	signIn(t, s, one, "r@example.test", "correct horse battery staple")
	if hashExists(t, db, one, HashToken(stale)) {
		t.Error("the sweep did not run again after an hour")
	}
}

// failingSweep is a Transactor whose third call — the sweep's own
// transaction, after a fresh Service's credentials read and session write —
// fails, while every other call runs on the real database.
type failingSweep struct {
	serving
	calls int
}

func (f *failingSweep) InTxFor(ctx context.Context, marketplaceID string, fn func(pgx.Tx) error) error {
	f.calls++
	if f.calls == 3 {
		return errors.New("sweep transaction failed")
	}
	return f.serving.InTxFor(ctx, marketplaceID, fn)
}

// A sweep failure is logged and never fails the sign-in: the person already
// has their session.
func TestASweepFailureDoesNotFailSignIn(t *testing.T) {
	db, one, _ := twoMarketplaces(t)
	setup := NewService(db, NewHasher(cheap, 1), breachedNone, &recording{}, quiet)
	confirmed(t, setup, db, one, "r@example.test")

	var logged bytes.Buffer
	s := NewService(&failingSweep{serving: db}, NewHasher(cheap, 1), breachedNone, &recording{},
		slog.New(slog.NewTextHandler(&logged, nil)))
	token, err := s.SignIn(t.Context(), visit(one), "r@example.test", "correct horse battery staple")
	if err != nil {
		t.Fatalf("SignIn = %v, want nil even though the sweep failed", err)
	}
	if _, err := s.Authenticate(t.Context(), one, token); err != nil {
		t.Fatalf("the session from a sign-in whose sweep failed does not authenticate: %v", err)
	}
	if !strings.Contains(logged.String(), "session sweep failed") {
		t.Errorf("the sweep failure left no trace in the log: %q", logged.String())
	}
}
