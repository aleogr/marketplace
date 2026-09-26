//go:build integration

package identity

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/aleogr/marketplace/internal/identity/breached"
	"github.com/aleogr/marketplace/internal/platform/audit"
)

// failures is how many second-factor answers in a row the account with
// address email has failed, as the service stored it: none when there is no
// row.
func failures(t *testing.T, db serving, marketplace, email string) int {
	t.Helper()
	var n int
	if err := db.InTxFor(t.Context(), marketplace, func(tx pgx.Tx) error {
		return tx.QueryRow(t.Context(), `
			SELECT coalesce((SELECT f.failures FROM second_factor_failure f JOIN account a ON a.id = f.account_id
			                  WHERE a.email_normalised = $1), 0)`, email).Scan(&n)
	}); err != nil {
		t.Fatal(err)
	}
	return n
}

// setFailures stores n failures in a row for the account with address email,
// as a hundred challenges would leave them, without the hundred challenges.
func setFailures(t *testing.T, db serving, marketplace, email string, n int) {
	t.Helper()
	if err := db.InTxFor(t.Context(), marketplace, func(tx pgx.Tx) error {
		_, err := tx.Exec(t.Context(), `
			INSERT INTO second_factor_failure (account_id, marketplace_id, failures, updated_at)
			SELECT id, marketplace_id, $2, now() FROM account WHERE email_normalised = $1
			ON CONFLICT (account_id) DO UPDATE SET failures = EXCLUDED.failures`, email, n)
		return err
	}); err != nil {
		t.Fatal(err)
	}
}

// failApp answers a fresh challenge of the account with a wrong app code, n
// times: one challenge each, so no challenge runs out of attempts.
func failApp(t *testing.T, s *Service, marketplace, email string, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		if _, err := s.CompleteSignIn(t.Context(), visit(marketplace), challenged(t, s, marketplace, email),
			Answer{Method: MethodApp, Code: "000000"}); !errors.Is(err, ErrCodeWrong) {
			t.Fatalf("wrong app code %d = %v, want ErrCodeWrong", i+1, err)
		}
	}
}

// audited counts the entries recorded for action.
func (r *recording) audited(action string) int {
	n := 0
	for _, e := range r.entries {
		if e.Action == action {
			n++
		}
	}
	return n
}

// The tenth failed second factor in a row tells the owner, once: the ninth
// does not, and neither does the eleventh (D8).
func TestTheTenthFailureInARowTellsTheOwnerOnce(t *testing.T) {
	s, db, one, _, trail := service(t)
	sealed(t, s)
	withApp(t, s, db, one, "r@example.test")

	failApp(t, s, one, "r@example.test", FailuresNotified-1)
	if got := noticeCount(t, db, one, "second-factor-failures"); got != 0 {
		t.Fatalf("second-factor-failures after %d failures = %d, want 0", FailuresNotified-1, got)
	}
	if got := trail.audited("identity.second_factor_failures_notified"); got != 0 {
		t.Fatalf("failures_notified audited %d times before the tenth failure", got)
	}

	failApp(t, s, one, "r@example.test", 1)
	if got := noticeCount(t, db, one, "second-factor-failures"); got != 1 {
		t.Fatalf("second-factor-failures after the tenth failure = %d, want 1", got)
	}
	if to, variables := mailed(t, db, one, "second-factor-failures"); to != "r@example.test" || variables["Name"] != "Reader" {
		t.Fatalf("second-factor-failures to %s with %v", to, variables)
	}
	notified := trail.entry(t, "identity.second_factor_failures_notified")
	if notified.Actor.ID != "" || notified.Subject.Person != "" || string(notified.After) != `{"failures":"10","user_agent":"test"}` {
		t.Fatalf("failures_notified recorded as %+v, %s", notified, notified.After)
	}

	failApp(t, s, one, "r@example.test", 1)
	if got := noticeCount(t, db, one, "second-factor-failures"); got != 1 {
		t.Fatalf("second-factor-failures after the eleventh failure = %d, want still 1", got)
	}
	if got := failures(t, db, one, "r@example.test"); got != FailuresNotified+1 {
		t.Fatalf("the count after eleven failures = %d", got)
	}
}

// A right answer ends the run: after nine failures and a success, it takes
// ten more failures, not one, to tell the owner.
func TestARightAnswerStartsTheCountAgain(t *testing.T) {
	s, db, one, _, _ := service(t)
	sealed(t, s)
	enrolment, _ := withApp(t, s, db, one, "r@example.test")

	failApp(t, s, one, "r@example.test", FailuresNotified-1)
	if _, err := s.CompleteSignIn(t.Context(), visit(one), challenged(t, s, one, "r@example.test"),
		Answer{Method: MethodApp, Code: nextAppCode(t, s, enrolment)}); err != nil {
		t.Fatalf("the right app code = %v", err)
	}
	if got := failures(t, db, one, "r@example.test"); got != 0 {
		t.Fatalf("the count after a right answer = %d, want 0", got)
	}

	failApp(t, s, one, "r@example.test", FailuresNotified-1)
	if got := noticeCount(t, db, one, "second-factor-failures"); got != 0 {
		t.Fatalf("second-factor-failures after a success and %d failures = %d, want 0", FailuresNotified-1, got)
	}
	failApp(t, s, one, "r@example.test", 1)
	if got := noticeCount(t, db, one, "second-factor-failures"); got != 1 {
		t.Fatalf("second-factor-failures after ten failures since the success = %d, want 1", got)
	}
}

// Every failed second factor counts — a wrong recovery code, a key's refused
// answer, a wrong e-mail factor code — and a challenge that cannot be
// answered does not, because no answer was checked.
func TestEveryFailedSecondFactorCounts(t *testing.T) {
	s, db, one, _, _ := service(t)
	sealed(t, s)
	session, key, _ := withKey(t, s, db, one, "r@example.test")
	v := visit(one)
	// The session predates the key, so it adds e-mail without a step-up.
	if err := s.BeginEmail(t.Context(), v, session); err != nil {
		t.Fatal(err)
	}
	if err := s.ConfirmEmail(t.Context(), v, session, lastCode(t, db, one)); err != nil {
		t.Fatal(err)
	}

	if _, err := s.CompleteSignIn(t.Context(), v, challenged(t, s, one, "r@example.test"),
		Answer{Method: MethodRecovery, Code: "0000-0000-0000"}); !errors.Is(err, ErrCodeWrong) {
		t.Fatalf("a wrong recovery code = %v, want ErrCodeWrong", err)
	}
	if got := failures(t, db, one, "r@example.test"); got != 1 {
		t.Fatalf("the count after a wrong recovery code = %d, want 1", got)
	}

	first := challenged(t, s, one, "r@example.test")
	stale, err := s.KeyOptions(t.Context(), v, first, "")
	if err != nil {
		t.Fatal(err)
	}
	second := challenged(t, s, one, "r@example.test")
	if _, err := s.KeyOptions(t.Context(), v, second, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CompleteSignIn(t.Context(), v, second, Answer{Method: MethodKey, Key: key.assert(stale)}); !errors.Is(err, ErrCodeWrong) {
		t.Fatalf("a key's answer to another challenge = %v, want ErrCodeWrong", err)
	}
	if got := failures(t, db, one, "r@example.test"); got != 2 {
		t.Fatalf("the count after a refused key = %d, want 2", got)
	}

	if _, err := s.CompleteSignIn(t.Context(), v, challenged(t, s, one, "r@example.test"),
		Answer{Method: MethodEmail, Code: "000000"}); !errors.Is(err, ErrCodeWrong) {
		t.Fatalf("a wrong e-mail code = %v, want ErrCodeWrong", err)
	}
	if got := failures(t, db, one, "r@example.test"); got != 3 {
		t.Fatalf("the count after a wrong e-mail factor code = %d, want 3", got)
	}

	if _, err := s.CompleteSignIn(t.Context(), v, "no such challenge", Answer{Method: MethodApp, Code: "000000"}); !errors.Is(err, ErrChallengeInvalid) {
		t.Fatalf("an unknown challenge = %v, want ErrChallengeInvalid", err)
	}
	if got := failures(t, db, one, "r@example.test"); got != 3 {
		t.Fatalf("the count after an unknown challenge = %d, want still 3", got)
	}
}

// storedStep is the last step the account's only app had a code accepted
// for.
func storedStep(t *testing.T, db serving, marketplace, email string) int64 {
	t.Helper()
	var step int64
	if err := db.InTxFor(t.Context(), marketplace, func(tx pgx.Tx) error {
		return tx.QueryRow(t.Context(), `
			SELECT f.totp_last_step FROM second_factor f JOIN account a ON a.id = f.account_id
			 WHERE a.email_normalised = $1 AND f.kind = 'totp'`, email).Scan(&step)
	}); err != nil {
		t.Fatal(err)
	}
	return step
}

// The hundredth failure in a row locks the codes (D8): the owner is told, the
// app is no longer offered, an app code is refused without being checked —
// the right one too — and a recovery code still signs in and lifts the lock.
func TestTheHundredthFailureLocksTheCodes(t *testing.T) {
	s, db, one, _, trail := service(t)
	sealed(t, s)
	enrolment, codes := withApp(t, s, db, one, "r@example.test")
	v := visit(one)

	setFailures(t, db, one, "r@example.test", FailuresLocked-2)
	failApp(t, s, one, "r@example.test", 1)
	if got := noticeCount(t, db, one, "second-factor-locked"); got != 0 {
		t.Fatalf("second-factor-locked after %d failures = %d, want 0", FailuresLocked-1, got)
	}
	failApp(t, s, one, "r@example.test", 1)
	if got := noticeCount(t, db, one, "second-factor-locked"); got != 1 {
		t.Fatalf("second-factor-locked after the hundredth failure = %d, want 1", got)
	}
	if to, variables := mailed(t, db, one, "second-factor-locked"); to != "r@example.test" || variables["Name"] != "Reader" {
		t.Fatalf("second-factor-locked to %s with %v", to, variables)
	}
	if locked := trail.entry(t, "identity.second_factor_locked"); string(locked.After) != `{"failures":"100","user_agent":"test"}` {
		t.Fatalf("second_factor_locked recorded as %s", locked.After)
	}
	if got := noticeCount(t, db, one, "second-factor-failures"); got != 0 {
		t.Fatalf("second-factor-failures, which the tenth failure sends, = %d on a count set past it", got)
	}

	token := challenged(t, s, one, "r@example.test")
	pending, err := s.Pending(t.Context(), v, token, "")
	if err != nil || len(pending.Methods) != 0 || !pending.Recovery || !pending.CodesLocked {
		t.Fatalf("Pending on a locked account = %+v, %v; want no app, a recovery code, and the lock", pending, err)
	}
	step := storedStep(t, db, one, "r@example.test")
	if _, err := s.CompleteSignIn(t.Context(), v, token, Answer{Method: MethodApp, Code: nextAppCode(t, s, enrolment)}); !errors.Is(err, ErrCodesLocked) || !errors.Is(err, ErrCodeWrong) {
		t.Fatalf("the right app code on a locked account = %v, want ErrCodesLocked", err)
	}
	if got := storedStep(t, db, one, "r@example.test"); got != step {
		t.Fatalf("the refused app code was checked: the last step moved from %d to %d", step, got)
	}
	if got := noticeCount(t, db, one, "second-factor-locked"); got != 1 {
		t.Fatalf("second-factor-locked after one more failure = %d, want still 1", got)
	}

	if _, err := s.CompleteSignIn(t.Context(), v, token, Answer{Method: MethodRecovery, Code: codes[0]}); err != nil {
		t.Fatalf("a recovery code on a locked account = %v", err)
	}
	if got := failures(t, db, one, "r@example.test"); got != 0 {
		t.Fatalf("the count after a recovery code = %d, want 0", got)
	}
	pending, err = s.Pending(t.Context(), v, challenged(t, s, one, "r@example.test"), "")
	if err != nil || !slices.Equal(pending.Methods, []Method{MethodApp}) || pending.CodesLocked {
		t.Fatalf("Pending once the lock is lifted = %+v, %v; want the app again", pending, err)
	}
}

// A key still signs in on a locked account, and lifts the lock; the codes
// an e-mail carries are locked like an app's, and none is even sent.
func TestAKeyLiftsTheLock(t *testing.T) {
	s, db, one, _, _ := service(t)
	sealed(t, s)
	session, key, _ := withKey(t, s, db, one, "r@example.test")
	v := visit(one)
	if err := s.BeginEmail(t.Context(), v, session); err != nil {
		t.Fatal(err)
	}
	if err := s.ConfirmEmail(t.Context(), v, session, lastCode(t, db, one)); err != nil {
		t.Fatal(err)
	}
	setFailures(t, db, one, "r@example.test", FailuresLocked)

	token := challenged(t, s, one, "r@example.test")
	pending, err := s.Pending(t.Context(), v, token, "")
	if err != nil || !slices.Equal(pending.Methods, []Method{MethodKey}) || !pending.CodesLocked {
		t.Fatalf("Pending on a locked account with a key and e-mail = %+v, %v; want the key only", pending, err)
	}
	later := time.Now().UTC().Add(emailCodeEvery)
	s.now = func() time.Time { return later }
	if err := s.SendChallengeCode(t.Context(), v, token, ""); !errors.Is(err, ErrCodesLocked) {
		t.Fatalf("a code for a locked account = %v, want ErrCodesLocked", err)
	}

	options, err := s.KeyOptions(t.Context(), v, token, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CompleteSignIn(t.Context(), v, token, Answer{Method: MethodKey, Key: key.assert(options)}); err != nil {
		t.Fatalf("the key on a locked account = %v", err)
	}
	if got := failures(t, db, one, "r@example.test"); got != 0 {
		t.Fatalf("the count after the key = %d, want 0", got)
	}
}

// The step-up is locked as the second step is: neither the app nor a code to
// the address is offered or accepted, a recovery code is, and it lifts the
// lock. A wrong app code at a step-up counts as one at sign-in does.
func TestTheStepUpIsLockedToo(t *testing.T) {
	s, db, one, _, _ := service(t)
	sealed(t, s)
	session := signedIn(t, s, db, one, "r@example.test")
	v := visit(one)
	enrolment, codes := enrolApp(t, s, v, session, "phone")
	session.SecondFactor = true

	challenge, err := s.BeginStepUp(t.Context(), v, session, ActionAddCard)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.StepUp(t.Context(), v, session, challenge, Answer{Method: MethodApp, Code: "000000"}); !errors.Is(err, ErrCodeWrong) {
		t.Fatalf("a wrong app code at a step-up = %v, want ErrCodeWrong", err)
	}
	if got := failures(t, db, one, "r@example.test"); got != 1 {
		t.Fatalf("the count after a wrong step-up = %d, want 1", got)
	}

	setFailures(t, db, one, "r@example.test", FailuresLocked)
	pending, err := s.Pending(t.Context(), v, challenge, session.ID)
	if err != nil || len(pending.Methods) != 0 || !pending.Recovery || !pending.CodesLocked {
		t.Fatalf("Pending of a locked step-up = %+v, %v; want a recovery code only", pending, err)
	}
	if err := s.StepUp(t.Context(), v, session, challenge, Answer{Method: MethodApp, Code: nextAppCode(t, s, enrolment)}); !errors.Is(err, ErrCodesLocked) {
		t.Fatalf("the right app code at a locked step-up = %v, want ErrCodesLocked", err)
	}
	if err := s.SendChallengeCode(t.Context(), v, challenge, session.ID); !errors.Is(err, ErrCodesLocked) {
		t.Fatalf("a code to the address at a locked step-up = %v, want ErrCodesLocked", err)
	}
	if err := s.StepUp(t.Context(), v, session, challenge, Answer{Method: MethodRecovery, Code: codes[0]}); err != nil {
		t.Fatalf("a recovery code at a locked step-up = %v", err)
	}
	if got := failures(t, db, one, "r@example.test"); got != 0 {
		t.Fatalf("the count after the recovery code = %d, want 0", got)
	}
}

// A code sent to the address for a card, when e-mail is not one of the
// account's factors, proves the address and not a second factor (D4): a
// wrong one does not count, and a right one does not end a run of failures.
func TestACodeToTheAddressIsNotASecondFactorAnswer(t *testing.T) {
	s, db, one, _, _ := service(t)
	sealed(t, s)
	session := signedIn(t, s, db, one, "r@example.test")
	v := visit(one)
	enrolApp(t, s, v, session, "phone")
	session.SecondFactor = true
	setFailures(t, db, one, "r@example.test", 5)

	challenge, err := s.BeginStepUp(t.Context(), v, session, ActionAddCard)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SendChallengeCode(t.Context(), v, challenge, session.ID); err != nil {
		t.Fatal(err)
	}
	code := lastCode(t, db, one)
	if err := s.StepUp(t.Context(), v, session, challenge, Answer{Method: MethodEmail, Code: otherCode(code)}); !errors.Is(err, ErrCodeWrong) {
		t.Fatalf("a wrong code to the address = %v, want ErrCodeWrong", err)
	}
	if got := failures(t, db, one, "r@example.test"); got != 5 {
		t.Fatalf("the count after a wrong code to the address = %d, want still 5", got)
	}
	if err := s.StepUp(t.Context(), v, session, challenge, Answer{Method: MethodEmail, Code: code}); err != nil {
		t.Fatalf("the code to the address = %v", err)
	}
	if got := failures(t, db, one, "r@example.test"); got != 5 {
		t.Fatalf("the count after the code to the address = %d, want still 5", got)
	}
}

// Changing the password lifts the lock and starts the count again (D8).
func TestChangingThePasswordStartsTheCountAgain(t *testing.T) {
	s, db, one, _, _ := service(t)
	sealed(t, s)
	session, token, enrolment := withAppSignedIn(t, s, db, one, "r@example.test")
	stepped := stepUp(t, s, one, token, session, ActionPassword, enrolment)
	setFailures(t, db, one, "r@example.test", FailuresLocked)

	if _, err := s.ChangePassword(t.Context(), visit(one), stepped, "correct horse battery staple", "a brand new passphrase"); err != nil {
		t.Fatalf("ChangePassword = %v", err)
	}
	if got := failures(t, db, one, "r@example.test"); got != 0 {
		t.Fatalf("the count after a password change = %d, want 0", got)
	}
}

// The count is the marketplace's, like the account: failures in one are
// invisible from the other, and an account of the same address there counts
// its own.
func TestTheCountIsTheMarketplacesOwn(t *testing.T) {
	s, db, one, two, _ := service(t)
	sealed(t, s)
	withApp(t, s, db, one, "r@example.test")
	withApp(t, s, db, two, "r@example.test")
	setFailures(t, db, one, "r@example.test", FailuresNotified-1)

	failApp(t, s, two, "r@example.test", 1)
	if got := failures(t, db, two, "r@example.test"); got != 1 {
		t.Fatalf("the count in marketplace two = %d, want 1", got)
	}
	if got := failures(t, db, one, "r@example.test"); got != FailuresNotified-1 {
		t.Fatalf("the count in marketplace one = %d, want %d", got, FailuresNotified-1)
	}
	if got := noticeCount(t, db, two, "second-factor-failures") + noticeCount(t, db, one, "second-factor-failures"); got != 0 {
		t.Fatalf("second-factor-failures = %d, want 0: neither account failed ten times", got)
	}

	var visible int
	if err := db.InTxFor(t.Context(), two, func(tx pgx.Tx) error {
		return tx.QueryRow(t.Context(), `SELECT count(*) FROM second_factor_failure`).Scan(&visible)
	}); err != nil {
		t.Fatal(err)
	}
	if visible != 1 {
		t.Fatalf("marketplace two sees %d failure rows, want its own one", visible)
	}
}

// Two failures counted at once, the first of them creating the row, count
// two: the second waits for the first and adds to what it left.
func TestTwoFailuresAtOnceCountTwo(t *testing.T) {
	_, db, one, _, _ := service(t)
	account := anAccount(t, db, one, "r@example.test")
	now := time.Now().UTC()

	holding := make(chan string, 1)
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseA := func() { releaseOnce.Do(func() { close(release) }) }
	defer releaseA()
	doneA := make(chan error, 1)
	go func() {
		doneA <- db.InTxFor(t.Context(), one, func(tx pgx.Tx) error {
			if _, err := countFailure(t.Context(), tx, one, account, now); err != nil {
				return err
			}
			var xid string
			if err := tx.QueryRow(t.Context(), `SELECT xid(pg_current_xact_id())::text`).Scan(&xid); err != nil {
				return err
			}
			holding <- xid
			<-release
			return nil
		})
	}()
	var xid string
	select {
	case xid = <-holding:
	case err := <-doneA:
		t.Fatalf("the first failure = %v", err)
	}

	var second int
	doneB := make(chan error, 1)
	go func() {
		doneB <- db.InTxFor(t.Context(), one, func(tx pgx.Tx) error {
			var err error
			second, err = countFailure(t.Context(), tx, one, account, now)
			return err
		})
	}()
	waitsFor(t, db, one, xid, doneB)
	releaseA()
	if err := <-doneA; err != nil {
		t.Fatal(err)
	}
	if err := <-doneB; err != nil {
		t.Fatal(err)
	}
	if second != 2 {
		t.Fatalf("the second failure counted %d, want 2", second)
	}
}

// guarded is a recording auditor safe for answers checked at once.
type guarded struct {
	mu sync.Mutex
	recording
}

func (g *guarded) Append(ctx context.Context, tx pgx.Tx, e audit.Entry) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.recording.Append(ctx, tx, e)
}

// Two wrong answers sent at once, to two challenges of one account, both
// count.
func TestTwoWrongAnswersAtOnceBothCount(t *testing.T) {
	_, db, one, _, _ := service(t)
	s := NewService(db, NewHasher(cheap, 2), breached.Fake{Known: breached.Common}, &guarded{}, quiet)
	sealed(t, s)
	withApp(t, s, db, one, "r@example.test")
	tokens := []string{challenged(t, s, one, "r@example.test"), challenged(t, s, one, "r@example.test")}

	start := make(chan struct{})
	results := make(chan error, len(tokens))
	for _, token := range tokens {
		go func() {
			<-start
			_, err := s.CompleteSignIn(t.Context(), visit(one), token, Answer{Method: MethodRecovery, Code: "0000-0000-0000"})
			results <- err
		}()
	}
	close(start)
	for range tokens {
		if err := <-results; !errors.Is(err, ErrCodeWrong) {
			t.Fatalf("a wrong answer = %v, want ErrCodeWrong", err)
		}
	}
	if got := failures(t, db, one, "r@example.test"); got != 2 {
		t.Fatalf("the count after two wrong answers at once = %d, want 2", got)
	}
}
