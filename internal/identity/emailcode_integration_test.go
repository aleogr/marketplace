//go:build integration

package identity

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

// lastCode is the code of the last second-factor-code mail requested in a
// marketplace, read from the outbox before it is dispatched.
func lastCode(t *testing.T, db serving, marketplace string) string {
	t.Helper()
	var code string
	if err := db.InTxFor(t.Context(), marketplace, func(tx pgx.Tx) error {
		return tx.QueryRow(t.Context(), `
			SELECT payload->'Variables'->>'Code' FROM outbox_event
			 WHERE kind = 'email.send' AND payload->>'Template' = 'second-factor-code'
			 ORDER BY created_at DESC, id DESC LIMIT 1`).Scan(&code)
	}); err != nil {
		t.Fatalf("no code was mailed: %v", err)
	}
	return code
}

// otherCode is a six-digit code that is not code.
func otherCode(code string) string {
	if code == "000000" {
		return "111111"
	}
	return "000000"
}

// withEmail is a signed-in account that receives its codes by e-mail.
func withEmail(t *testing.T, s *Service, db serving, marketplace, email string) Session {
	t.Helper()
	session := signedIn(t, s, db, marketplace, email)
	if err := s.BeginEmail(t.Context(), visit(marketplace), session); err != nil {
		t.Fatal(err)
	}
	if err := s.ConfirmEmail(t.Context(), visit(marketplace), session, lastCode(t, db, marketplace)); err != nil {
		t.Fatal(err)
	}
	return session
}

// E-mail becomes a second factor when the code sent to the address comes
// back; a wrong code adds nothing, and the account has one e-mail factor at
// most.
func TestAddingEmailProvesTheAddressReceivesTheCode(t *testing.T) {
	s, db, one, _, trail := service(t)
	sealed(t, s)
	session := signedIn(t, s, db, one, "r@example.test")
	v := visit(one)

	if err := s.BeginEmail(t.Context(), v, session); err != nil {
		t.Fatal(err)
	}
	code := lastCode(t, db, one)
	if err := s.ConfirmEmail(t.Context(), v, session, otherCode(code)); !errors.Is(err, ErrCodeWrong) {
		t.Fatalf("a wrong code = %v, want ErrCodeWrong", err)
	}
	if err := s.ConfirmEmail(t.Context(), v, session, code); err != nil {
		t.Fatalf("the code sent = %v", err)
	}
	security, err := s.Security(t.Context(), v, session)
	if err != nil || len(security.Factors) != 1 || security.Factors[0].Method != MethodEmail || security.RecoveryIssued != 0 {
		t.Fatalf("Security = %+v, %v; want one e-mail factor and no recovery codes", security, err)
	}
	if err := s.BeginEmail(t.Context(), v, session); !errors.Is(err, ErrAlreadyEnrolled) {
		t.Fatalf("adding e-mail twice = %v, want ErrAlreadyEnrolled", err)
	}
	trail.entry(t, "identity.second_factor_added")
}

// Staff cannot add an e-mail code, and are sent none (§18.2): the policy
// refuses before anything is written or mailed.
func TestAStaffAccountCannotEnrolAnEmailCode(t *testing.T) {
	s, db, one, _, _ := service(t)
	sealed(t, s)
	staff := signedIn(t, s, db, one, "staff@example.test")
	staff.Account.Kind = KindStaff
	now := s.now()
	staff.SteppedUpAt = &now
	mails := len(outbox(t, db, one))

	if err := s.BeginEmail(t.Context(), visit(one), staff); !errors.Is(err, ErrNotPermitted) {
		t.Fatalf("BeginEmail for staff = %v, want ErrNotPermitted", err)
	}
	if err := s.ConfirmEmail(t.Context(), visit(one), staff, "123456"); !errors.Is(err, ErrNotPermitted) {
		t.Fatalf("ConfirmEmail for staff = %v, want ErrNotPermitted", err)
	}
	if after := len(outbox(t, db, one)); after != mails {
		t.Fatalf("%d mails were requested for staff", after-mails)
	}
}

// Nor is an e-mail factor offered to staff at verification, even one already
// stored: the policy filters what a challenge accepts.
func TestAnEmailFactorIsNotOfferedToStaff(t *testing.T) {
	s, db, one, _, _ := service(t)
	sealed(t, s)
	session := withEmail(t, s, db, one, "r@example.test")
	staff := session.Account
	staff.Kind = KindStaff
	if err := db.InTxFor(t.Context(), one, func(tx pgx.Tx) error {
		methods, _, err := s.offers(t.Context(), tx, staff, challenge{Account: staff.ID})
		if err != nil {
			return err
		}
		if slices.Contains(methods, MethodEmail) {
			t.Errorf("staff are offered %v", methods)
		}
		buyer, _, err := s.offers(t.Context(), tx, session.Account, challenge{Account: staff.ID})
		if err != nil {
			return err
		}
		if !slices.Equal(buyer, []Method{MethodEmail}) {
			t.Errorf("the buyer is offered %v, want the e-mail code", buyer)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// The e-mail path of the second step: the code is sent on request and
// signs in once.
func TestSigningInWithAnEmailCode(t *testing.T) {
	s, db, one, _, _ := service(t)
	sealed(t, s)
	withEmail(t, s, db, one, "r@example.test")
	token := challenged(t, s, one, "r@example.test")
	pending, err := s.Pending(t.Context(), visit(one), token, "")
	if err != nil || !slices.Equal(pending.Methods, []Method{MethodEmail}) || pending.Recovery {
		t.Fatalf("Pending = %+v, %v", pending, err)
	}

	// The enrolment's code was sent less than a minute ago.
	if err := s.SendChallengeCode(t.Context(), visit(one), token, ""); !errors.Is(err, ErrCodeTooSoon) {
		t.Fatalf("a second code within a minute = %v, want ErrCodeTooSoon", err)
	}
	later := time.Now().UTC().Add(emailCodeEvery)
	s.now = func() time.Time { return later }
	if err := s.SendChallengeCode(t.Context(), visit(one), token, ""); err != nil {
		t.Fatal(err)
	}
	code := lastCode(t, db, one)
	signed, err := s.CompleteSignIn(t.Context(), visit(one), token, Answer{Method: MethodEmail, Code: code})
	if err != nil {
		t.Fatalf("CompleteSignIn with the e-mail code = %v", err)
	}
	if _, err := s.Authenticate(t.Context(), one, signed.Session); err != nil {
		t.Fatal(err)
	}
	again := challenged(t, s, one, "r@example.test")
	if _, err := s.CompleteSignIn(t.Context(), visit(one), again, Answer{Method: MethodEmail, Code: code}); !errors.Is(err, ErrCodeWrong) {
		t.Fatalf("the spent code again = %v, want ErrCodeWrong", err)
	}
}

// An e-mail code lives ten minutes.
func TestAnExpiredEmailCodeIsRefused(t *testing.T) {
	s, db, one, _, _ := service(t)
	sealed(t, s)
	session := signedIn(t, s, db, one, "r@example.test")
	if err := s.BeginEmail(t.Context(), visit(one), session); err != nil {
		t.Fatal(err)
	}
	later := time.Now().UTC().Add(EmailCodeLifetime + time.Second)
	s.now = func() time.Time { return later }
	if err := s.ConfirmEmail(t.Context(), visit(one), session, lastCode(t, db, one)); !errors.Is(err, ErrCodeWrong) {
		t.Fatalf("an expired code = %v, want ErrCodeWrong", err)
	}
}

// Five wrong attempts and the code is dead, even the right one after them.
func TestAnEmailCodeTakesFiveAttempts(t *testing.T) {
	s, db, one, _, _ := service(t)
	sealed(t, s)
	session := signedIn(t, s, db, one, "r@example.test")
	if err := s.BeginEmail(t.Context(), visit(one), session); err != nil {
		t.Fatal(err)
	}
	code := lastCode(t, db, one)
	for range EmailCodeAttempts {
		if err := s.ConfirmEmail(t.Context(), visit(one), session, otherCode(code)); !errors.Is(err, ErrCodeWrong) {
			t.Fatalf("a wrong code = %v", err)
		}
	}
	if err := s.ConfirmEmail(t.Context(), visit(one), session, code); !errors.Is(err, ErrCodeWrong) {
		t.Fatalf("the right code after five wrong ones = %v, want ErrCodeWrong", err)
	}
}

// One code a minute and five an hour per account, whatever their purpose.
func TestEmailCodesAreLimitedPerAccount(t *testing.T) {
	s, db, one, _, _ := service(t)
	sealed(t, s)
	session := signedIn(t, s, db, one, "r@example.test")
	start := time.Now().UTC()
	for sent := range emailCodesPerHour {
		at := start.Add(time.Duration(sent) * emailCodeEvery)
		s.now = func() time.Time { return at }
		if err := s.BeginEmail(t.Context(), visit(one), session); err != nil {
			t.Fatalf("code %d = %v", sent+1, err)
		}
		if err := s.BeginEmail(t.Context(), visit(one), session); !errors.Is(err, ErrCodeTooSoon) {
			t.Fatalf("a second code in the same minute = %v, want ErrCodeTooSoon", err)
		}
	}
	at := start.Add(emailCodesPerHour * emailCodeEvery)
	s.now = func() time.Time { return at }
	if err := s.BeginEmail(t.Context(), visit(one), session); !errors.Is(err, ErrCodeTooMany) {
		t.Fatalf("a sixth code within the hour = %v, want ErrCodeTooMany", err)
	}
	at = start.Add(time.Hour + time.Second)
	if err := s.BeginEmail(t.Context(), visit(one), session); err != nil {
		t.Fatalf("a code an hour later = %v", err)
	}
}

// A code has one purpose: the one that adds e-mail does not sign in.
func TestAnEmailCodeAnswersOnlyItsPurpose(t *testing.T) {
	s, db, one, _, _ := service(t)
	sealed(t, s)
	session := withEmail(t, s, db, one, "r@example.test")
	// A second address factor cannot be added, so a fresh enrolment code is
	// sent the way BeginEmail would, for the test's sake.
	later := time.Now().UTC().Add(emailCodeEvery)
	s.now = func() time.Time { return later }
	if err := db.InTxFor(t.Context(), one, func(tx pgx.Tx) error {
		return s.sendCode(t.Context(), tx, visit(one), session.Account, purposeEnrol, later)
	}); err != nil {
		t.Fatal(err)
	}
	token := challenged(t, s, one, "r@example.test")
	if _, err := s.CompleteSignIn(t.Context(), visit(one), token, Answer{Method: MethodEmail, Code: lastCode(t, db, one)}); !errors.Is(err, ErrCodeWrong) {
		t.Fatalf("an enrolment code at sign-in = %v, want ErrCodeWrong", err)
	}
}

// A buyer with no 2FA answers the step-up before adding a card with a code
// to their own address (§18.2).
func TestABuyerStepsUpBeforeACardWithAnEmailCode(t *testing.T) {
	s, db, one, _, trail := service(t)
	sealed(t, s)
	buyer := signedIn(t, s, db, one, "r@example.test")
	token, err := s.BeginStepUp(t.Context(), visit(one), buyer, ActionAddCard)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SendChallengeCode(t.Context(), visit(one), token, buyer.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.StepUp(t.Context(), visit(one), buyer, token, Answer{Method: MethodEmail, Code: lastCode(t, db, one)}); err != nil {
		t.Fatalf("StepUp with the e-mail code = %v", err)
	}
	if stepped := trail.entry(t, "identity.stepped_up"); string(stepped.After) != `{"action":"add_card","method":"email","user_agent":"test"}` {
		t.Fatalf("stepped_up recorded as %s", stepped.After)
	}
}

// waitBlocked returns once n transactions of this database wait on a lock,
// read from tx, which holds the lock they wait for; it fails the test after
// ten seconds.
func waitBlocked(t *testing.T, tx pgx.Tx, n int) error {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		// The activity view is a snapshot taken once per transaction: it is
		// cleared so each look sees the backends that connected since.
		if _, err := tx.Exec(t.Context(), `SELECT pg_stat_clear_snapshot()`); err != nil {
			return err
		}
		var blocked int
		if err := tx.QueryRow(t.Context(), `
			SELECT count(*) FROM pg_stat_activity
			 WHERE datname = current_database() AND cardinality(pg_blocking_pids(pid)) > 0`).Scan(&blocked); err != nil {
			return err
		}
		if blocked >= n {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("%d transactions waited on a lock after ten seconds, want %d", blocked, n)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// Two codes asked for one account at the same moment: the account is locked
// before the limits are counted, so the second waits for the first to
// commit, counts its code, and is refused (D6). Without the lock both would
// count none and both would be mailed.
func TestCodesAskedForAtOnceAreLimitedPerAccount(t *testing.T) {
	s, db, one, _, _ := service(t)
	sealed(t, s)
	session := signedIn(t, s, db, one, "r@example.test")
	mails := len(outbox(t, db, one))

	results := make(chan error, 2)
	if err := db.InTxFor(t.Context(), one, func(tx pgx.Tx) error {
		// Holding the account's row stops both sends where they first touch
		// it: taking its lock, or, with none taken, the foreign-key check of
		// the code they insert after counting. Once both wait, it lets go.
		if _, err := tx.Exec(t.Context(), `SELECT 1 FROM account WHERE id = $1 FOR UPDATE`, session.Account.ID); err != nil {
			return err
		}
		for range 2 {
			go func() { results <- s.BeginEmail(context.Background(), visit(one), session) }()
		}
		return waitBlocked(t, tx, 2)
	}); err != nil {
		t.Fatal(err)
	}

	errs := []error{<-results, <-results}
	sent := slices.IndexFunc(errs, func(err error) bool { return err == nil })
	refused := slices.IndexFunc(errs, func(err error) bool { return errors.Is(err, ErrCodeTooSoon) })
	if sent < 0 || refused < 0 {
		t.Fatalf("two codes asked for at once = %v, want one sent and one ErrCodeTooSoon", errs)
	}
	if after := len(outbox(t, db, one)); after != mails+1 {
		t.Fatalf("%d codes were mailed, want 1", after-mails)
	}
}

// A sign-in with an e-mail code records when the e-mail factor was last used,
// as an app's code does, so the security page can say it.
func TestSigningInWithAnEmailCodeRecordsItsLastUse(t *testing.T) {
	s, db, one, _, _ := service(t)
	sealed(t, s)
	session := withEmail(t, s, db, one, "last-used@example.test")
	token := challenged(t, s, one, "last-used@example.test")
	later := time.Now().UTC().Add(emailCodeEvery)
	s.now = func() time.Time { return later }
	if err := s.SendChallengeCode(t.Context(), visit(one), token, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CompleteSignIn(t.Context(), visit(one), token,
		Answer{Method: MethodEmail, Code: lastCode(t, db, one)}); err != nil {
		t.Fatalf("CompleteSignIn with the e-mail code = %v", err)
	}

	security, err := s.Security(t.Context(), visit(one), session)
	if err != nil || len(security.Factors) != 1 || security.Factors[0].Method != MethodEmail {
		t.Fatalf("Security = %+v, %v; want the e-mail factor", security, err)
	}
	if used := security.Factors[0].LastUsedAt; used == nil || used.Sub(later).Abs() > time.Millisecond {
		t.Fatalf("the e-mail factor was last used at %v, want %v", used, later)
	}
}
