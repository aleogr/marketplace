//go:build integration

package identity

import (
	"errors"
	"slices"
	"testing"
	"time"
)

// withAppSignedIn is an account with an app and a session opened before the
// app was added, read again as a request after it would read it: with 2FA,
// and no step-up.
func withAppSignedIn(t *testing.T, s *Service, db serving, marketplace, email string) (Session, string, AppEnrolment) {
	t.Helper()
	confirmed(t, s, db, marketplace, email)
	token := signIn(t, s, marketplace, email, "correct horse battery staple")
	session, err := s.Authenticate(t.Context(), marketplace, token)
	if err != nil {
		t.Fatal(err)
	}
	enrolment, _ := enrolApp(t, s, visit(marketplace), session, "phone")
	if session, err = s.Authenticate(t.Context(), marketplace, token); err != nil || !session.SecondFactor {
		t.Fatalf("the session after the app was added = %+v, %v", session, err)
	}
	return session, token, enrolment
}

// stepUp proves the app for session before action and returns the session
// as the next request reads it.
func stepUp(t *testing.T, s *Service, marketplace, token string, session Session, action Action, enrolment AppEnrolment) Session {
	t.Helper()
	challenge, err := s.BeginStepUp(t.Context(), visit(marketplace), session, action)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.StepUp(t.Context(), visit(marketplace), session, challenge,
		Answer{Method: MethodApp, Code: nextAppCode(t, s, enrolment)}); err != nil {
		t.Fatalf("StepUp = %v", err)
	}
	stepped, err := s.Authenticate(t.Context(), marketplace, token)
	if err != nil || stepped.SteppedUpAt == nil {
		t.Fatalf("the session after the step-up = %+v, %v", stepped, err)
	}
	return stepped
}

// Once the account has 2FA, changing its factors asks for the second factor
// again (D2), and a step-up lets the change through.
func TestChangingFactorsAsksForARecentStepUp(t *testing.T) {
	s, db, one, _, trail := service(t)
	sealed(t, s)
	session, token, enrolment := withAppSignedIn(t, s, db, one, "r@example.test")
	v := visit(one)
	security, err := s.Security(t.Context(), v, session)
	if err != nil {
		t.Fatal(err)
	}

	if err := s.RemoveFactor(t.Context(), v, session, security.Factors[0].ID); !errors.Is(err, ErrStepUpNeeded) {
		t.Fatalf("RemoveFactor with no step-up = %v, want ErrStepUpNeeded", err)
	}
	if _, err := s.BeginApp(t.Context(), v, session); !errors.Is(err, ErrStepUpNeeded) {
		t.Fatalf("BeginApp with no step-up = %v, want ErrStepUpNeeded", err)
	}
	if _, err := s.RegenerateRecoveryCodes(t.Context(), v, session); !errors.Is(err, ErrStepUpNeeded) {
		t.Fatalf("RegenerateRecoveryCodes with no step-up = %v, want ErrStepUpNeeded", err)
	}

	challenge, err := s.BeginStepUp(t.Context(), v, session, ActionFactors)
	if err != nil {
		t.Fatal(err)
	}
	pending, err := s.Pending(t.Context(), v, challenge, session.ID)
	if err != nil || pending.Action != ActionFactors || !slices.Equal(pending.Methods, []Method{MethodApp}) {
		t.Fatalf("Pending = %+v, %v", pending, err)
	}
	if err := s.StepUp(t.Context(), v, session, challenge, Answer{Method: MethodApp, Code: nextAppCode(t, s, enrolment)}); err != nil {
		t.Fatalf("StepUp = %v", err)
	}
	stepped, err := s.Authenticate(t.Context(), one, token)
	if err != nil || stepped.SteppedUpAt == nil {
		t.Fatalf("the session after the step-up = %+v, %v", stepped, err)
	}
	if err := s.RemoveFactor(t.Context(), v, stepped, security.Factors[0].ID); err != nil {
		t.Fatalf("RemoveFactor after the step-up = %v", err)
	}
	if stepped := trail.entry(t, "identity.stepped_up"); string(stepped.After) != `{"action":"factors","method":"totp","user_agent":"test"}` {
		t.Fatalf("stepped_up recorded as %s", stepped.After)
	}
}

// A step-up lasts ten minutes (D4).
func TestAStepUpOlderThanTenMinutesIsRefused(t *testing.T) {
	s, db, one, _, _ := service(t)
	sealed(t, s)
	session, token, enrolment := withAppSignedIn(t, s, db, one, "r@example.test")
	stepped := stepUp(t, s, one, token, session, ActionFactors, enrolment)
	if s.NeedsStepUp(stepped, ActionFactors) {
		t.Fatal("a step-up just made is not enough")
	}

	later := time.Now().UTC().Add(StepUpLifetime)
	s.now = func() time.Time { return later }
	if !s.NeedsStepUp(stepped, ActionFactors) {
		t.Fatal("a step-up ten minutes old is still enough")
	}
	if _, err := s.RegenerateRecoveryCodes(t.Context(), visit(one), stepped); !errors.Is(err, ErrStepUpNeeded) {
		t.Fatalf("a change ten minutes after the step-up = %v, want ErrStepUpNeeded", err)
	}
}

// Whoever has a second factor proves it before changing the password (D2),
// and the session the change opens keeps that step-up.
func TestThePasswordChangeAsksForTheSecondFactor(t *testing.T) {
	s, db, one, _, _ := service(t)
	sealed(t, s)
	session, token, enrolment := withAppSignedIn(t, s, db, one, "r@example.test")
	v := visit(one)
	if _, err := s.ChangePassword(t.Context(), v, session, "correct horse battery staple", "a brand new passphrase"); !errors.Is(err, ErrStepUpNeeded) {
		t.Fatalf("ChangePassword with no step-up = %v, want ErrStepUpNeeded", err)
	}
	stepped := stepUp(t, s, one, token, session, ActionPassword, enrolment)
	renewed, err := s.ChangePassword(t.Context(), v, stepped, "correct horse battery staple", "a brand new passphrase")
	if err != nil {
		t.Fatalf("ChangePassword after the step-up = %v", err)
	}
	after, err := s.Authenticate(t.Context(), one, renewed)
	if err != nil || after.SteppedUpAt == nil || !after.SteppedUpAt.Equal(*stepped.SteppedUpAt) {
		t.Fatalf("the renewed session = %+v, %v; want the step-up carried over", after, err)
	}
}

// A step-up's challenge is answered by the session that opened it, and by no
// other: not another session of the same account, not a sign-in.
func TestAStepUpChallengeIsItsSessionsOnly(t *testing.T) {
	s, db, one, _, _ := service(t)
	sealed(t, s)
	session, _, enrolment := withAppSignedIn(t, s, db, one, "r@example.test")
	v := visit(one)
	challenge, err := s.BeginStepUp(t.Context(), v, session, ActionFactors)
	if err != nil {
		t.Fatal(err)
	}
	other := session
	other.ID = "00000000-0000-0000-0000-000000000001"
	if err := s.StepUp(t.Context(), v, other, challenge, Answer{Method: MethodApp, Code: nextAppCode(t, s, enrolment)}); !errors.Is(err, ErrChallengeInvalid) {
		t.Fatalf("another session answering = %v, want ErrChallengeInvalid", err)
	}
	if _, err := s.CompleteSignIn(t.Context(), v, challenge, Answer{Method: MethodApp, Code: nextAppCode(t, s, enrolment)}); !errors.Is(err, ErrChallengeInvalid) {
		t.Fatalf("a sign-in answering a step-up = %v, want ErrChallengeInvalid", err)
	}
}

// The step-up entry points of adding a card and changing the address exist
// for the deliveries that add those actions: a buyer is asked even without
// 2FA, and may answer with a code to their own address (§18.2); staff, who
// may not use e-mail, have nothing to answer with until they have a factor.
func TestTheStepUpEntryPointsForACardAndAnAddress(t *testing.T) {
	s, db, one, _, _ := service(t)
	sealed(t, s)
	buyer := signedIn(t, s, db, one, "r@example.test")
	v := visit(one)
	for _, action := range []Action{ActionAddCard, ActionChangeEmail} {
		if !s.NeedsStepUp(buyer, action) {
			t.Errorf("a buyer with no 2FA is not asked before %s", action)
		}
		challenge, err := s.BeginStepUp(t.Context(), v, buyer, action)
		if err != nil {
			t.Fatal(err)
		}
		pending, err := s.Pending(t.Context(), v, challenge, buyer.ID)
		if err != nil || !slices.Equal(pending.Methods, []Method{MethodEmail}) || pending.Action != action {
			t.Errorf("Pending before %s = %+v, %v; want the e-mail code", action, pending, err)
		}
	}
	if s.NeedsStepUp(buyer, ActionPassword) {
		t.Error("a buyer with no 2FA is asked before changing the password")
	}

	staff := buyer
	staff.Account.Kind = KindStaff
	if _, err := s.BeginStepUp(t.Context(), v, staff, ActionAddCard); !errors.Is(err, ErrNoSecondFactor) {
		t.Fatalf("staff with no factor stepping up = %v, want ErrNoSecondFactor", err)
	}
}

// Staff must end up with a second factor (F15 enforces that at sign-in), but
// a step-up is only asked of whoever already has one (D2): a staff account
// with none yet is not asked before enrolling its first, or it could never
// enrol one at all. It still has nothing to step up with for anything else.
func TestStaffWithNoFactorMayEnrolTheirFirst(t *testing.T) {
	s, db, one, _, _ := service(t)
	sealed(t, s)
	session := signedIn(t, s, db, one, "r@example.test")
	v := visit(one)

	staff := session
	staff.Account.Kind = KindStaff
	if s.NeedsStepUp(staff, ActionFactors) {
		t.Error("staff with no factor is asked to step up before adding their first one")
	}
	if _, err := s.BeginApp(t.Context(), v, staff); errors.Is(err, ErrStepUpNeeded) {
		t.Errorf("BeginApp for staff with no factor = %v, want no ErrStepUpNeeded", err)
	}

	if _, err := s.BeginStepUp(t.Context(), v, staff, ActionAddCard); !errors.Is(err, ErrNoSecondFactor) {
		t.Fatalf("staff with no factor stepping up for %s = %v, want ErrNoSecondFactor", ActionAddCard, err)
	}
}
