//go:build integration

package identity

import (
	"errors"
	"slices"
	"testing"
	"time"
)

// nextAppCode is the code an app shows one step from now. Enrolment spent
// the current step, and a step is never accepted twice (D6); the next one is
// within the drift.
func nextAppCode(t *testing.T, s *Service, enrolment AppEnrolment) string {
	t.Helper()
	secret, err := totpEncoding.DecodeString(enrolment.Key)
	if err != nil {
		t.Fatal(err)
	}
	return totpCode(secret, totpStep(s.now())+1)
}

// withApp is a confirmed account with an app, and what the app was set up
// with and the recovery codes it showed.
func withApp(t *testing.T, s *Service, db serving, marketplace, email string) (AppEnrolment, []string) {
	t.Helper()
	session := signedIn(t, s, db, marketplace, email)
	return enrolApp(t, s, visit(marketplace), session, "phone")
}

// challenged signs in with the password and returns the challenge's token.
func challenged(t *testing.T, s *Service, marketplace, email string) string {
	t.Helper()
	token, err := s.SignIn(t.Context(), visit(marketplace), email, "correct horse battery staple")
	if !errors.Is(err, ErrSecondStep) || token == "" {
		t.Fatalf("SignIn on an account with an app = %q, %v; want a challenge and ErrSecondStep", token, err)
	}
	return token
}

// A correct password on an account with a second factor opens a challenge,
// never a session (D3); the app's code opens the session, stepped up, and the
// challenge is spent.
func TestSigningInWithAnAppTakesTwoSteps(t *testing.T) {
	s, db, one, _, trail := service(t)
	sealed(t, s)
	enrolment, _ := withApp(t, s, db, one, "r@example.test")

	token := challenged(t, s, one, "r@example.test")
	if _, err := s.Authenticate(t.Context(), one, token); !errors.Is(err, ErrSessionInvalid) {
		t.Fatalf("the challenge's token opens a session: %v", err)
	}
	pending, err := s.Pending(t.Context(), visit(one), token, "")
	if err != nil || !slices.Equal(pending.Methods, []Method{MethodApp}) || !pending.Recovery || pending.Action != "" {
		t.Fatalf("Pending = %+v, %v", pending, err)
	}

	signed, err := s.CompleteSignIn(t.Context(), visit(one), token, Answer{Method: MethodApp, Code: nextAppCode(t, s, enrolment)})
	if err != nil || signed.Recovery {
		t.Fatalf("CompleteSignIn = %+v, %v", signed, err)
	}
	session, err := s.Authenticate(t.Context(), one, signed.Session)
	if err != nil || !session.SecondFactor || session.SteppedUpAt == nil {
		t.Fatalf("the session = %+v, %v; want it with 2FA and stepped up", session, err)
	}
	if _, err := s.CompleteSignIn(t.Context(), visit(one), token, Answer{Method: MethodApp, Code: nextAppCode(t, s, enrolment)}); !errors.Is(err, ErrChallengeInvalid) {
		t.Fatalf("a spent challenge = %v, want ErrChallengeInvalid", err)
	}

	var signin bool
	for _, e := range trail.entries {
		if e.Action == "identity.signin" && string(e.After) == `{"method":"totp","user_agent":"test"}` {
			signin = true
		}
	}
	if !signin {
		t.Fatalf("no identity.signin naming the app among %v", trail.actions())
	}
}

// The code the app showed when it was added cannot sign in: its step was
// accepted once already (D6).
func TestAnAppCodeIsNeverAcceptedTwice(t *testing.T) {
	s, db, one, _, _ := service(t)
	sealed(t, s)
	session := signedIn(t, s, db, one, "r@example.test")
	enrolment, err := s.BeginApp(t.Context(), visit(one), session)
	if err != nil {
		t.Fatal(err)
	}
	code := appCode(t, s, enrolment)
	if _, err := s.ConfirmApp(t.Context(), visit(one), session, "phone", code); err != nil {
		t.Fatal(err)
	}
	token := challenged(t, s, one, "r@example.test")
	if _, err := s.CompleteSignIn(t.Context(), visit(one), token, Answer{Method: MethodApp, Code: code}); !errors.Is(err, ErrCodeWrong) {
		t.Fatalf("the enrolment's code again = %v, want ErrCodeWrong", err)
	}
}

// Four wrong answers leave the challenge open; the fifth destroys it, and the
// right answer after that is refused (D3). Each failure is audited.
func TestTheFifthWrongAnswerEndsTheChallenge(t *testing.T) {
	s, db, one, _, trail := service(t)
	sealed(t, s)
	enrolment, _ := withApp(t, s, db, one, "r@example.test")
	token := challenged(t, s, one, "r@example.test")
	wrong := Answer{Method: MethodApp, Code: "000000"}

	for attempt := 1; attempt < ChallengeAttempts; attempt++ {
		if _, err := s.CompleteSignIn(t.Context(), visit(one), token, wrong); !errors.Is(err, ErrCodeWrong) {
			t.Fatalf("wrong answer %d = %v, want ErrCodeWrong", attempt, err)
		}
	}
	if _, err := s.CompleteSignIn(t.Context(), visit(one), token, wrong); !errors.Is(err, ErrChallengeExhausted) {
		t.Fatalf("the fifth wrong answer = %v, want ErrChallengeExhausted", err)
	}
	right := Answer{Method: MethodApp, Code: nextAppCode(t, s, enrolment)}
	if _, err := s.CompleteSignIn(t.Context(), visit(one), token, right); !errors.Is(err, ErrChallengeInvalid) {
		t.Fatalf("the right answer after the fifth wrong one = %v, want ErrChallengeInvalid", err)
	}
	failed := trail.entry(t, "identity.second_factor_failed")
	if failed.Actor.ID != "" || failed.Subject.Person != "" || failed.From != "203.0.113.7" {
		t.Fatalf("second_factor_failed recorded as %+v", failed)
	}
}

// A challenge lives five minutes.
func TestAnExpiredChallengeIsRefused(t *testing.T) {
	s, db, one, _, _ := service(t)
	sealed(t, s)
	enrolment, _ := withApp(t, s, db, one, "r@example.test")
	token := challenged(t, s, one, "r@example.test")

	later := time.Now().UTC().Add(ChallengeLifetime + time.Second)
	s.now = func() time.Time { return later }
	if _, err := s.Pending(t.Context(), visit(one), token, ""); !errors.Is(err, ErrChallengeInvalid) {
		t.Fatalf("Pending on an expired challenge = %v, want ErrChallengeInvalid", err)
	}
	if _, err := s.CompleteSignIn(t.Context(), visit(one), token, Answer{Method: MethodApp, Code: nextAppCode(t, s, enrolment)}); !errors.Is(err, ErrChallengeInvalid) {
		t.Fatalf("an expired challenge = %v, want ErrChallengeInvalid", err)
	}
}

// A recovery code signs in once, says how many are left, and is audited.
func TestARecoveryCodeSignsInOnce(t *testing.T) {
	s, db, one, _, trail := service(t)
	sealed(t, s)
	_, codes := withApp(t, s, db, one, "r@example.test")

	signed, err := s.CompleteSignIn(t.Context(), visit(one), challenged(t, s, one, "r@example.test"),
		Answer{Method: MethodRecovery, Code: codes[3]})
	if err != nil || !signed.Recovery || signed.RecoveryLeft != RecoveryCodeCount-1 {
		t.Fatalf("CompleteSignIn with a recovery code = %+v, %v; want %d left", signed, err, RecoveryCodeCount-1)
	}
	if _, err := s.CompleteSignIn(t.Context(), visit(one), challenged(t, s, one, "r@example.test"),
		Answer{Method: MethodRecovery, Code: codes[3]}); !errors.Is(err, ErrCodeWrong) {
		t.Fatalf("the same recovery code again = %v, want ErrCodeWrong", err)
	}
	if used := trail.entry(t, "identity.recovery_code_used"); string(used.After) != `{"left":"9","user_agent":"test"}` {
		t.Fatalf("recovery_code_used recorded as %s", used.After)
	}
}

// A challenge is its marketplace's, like everything else of the account.
func TestAChallengeIsInvisibleFromAnotherMarketplace(t *testing.T) {
	s, db, one, two, _ := service(t)
	sealed(t, s)
	enrolment, _ := withApp(t, s, db, one, "r@example.test")
	token := challenged(t, s, one, "r@example.test")
	if _, err := s.CompleteSignIn(t.Context(), visit(two), token, Answer{Method: MethodApp, Code: nextAppCode(t, s, enrolment)}); !errors.Is(err, ErrChallengeInvalid) {
		t.Fatalf("answering from another marketplace = %v, want ErrChallengeInvalid", err)
	}
	if address := s.ChallengeAddress(t.Context(), visit(two), token); address != "" {
		t.Fatalf("another marketplace reads the challenge's address %q", address)
	}
	if address := s.ChallengeAddress(t.Context(), visit(one), token); address != "r@example.test" {
		t.Fatalf("ChallengeAddress = %q, want the account's", address)
	}
}
