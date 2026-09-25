//go:build integration

package identity

import (
	"errors"
	"slices"
	"testing"
)

// withKey is a signed-in account with a software key added through the
// service, and the key.
func withKey(t *testing.T, s *Service, db serving, marketplace, email string) (Session, *softKey, []string) {
	t.Helper()
	session := signedIn(t, s, db, marketplace, email)
	v := visit(marketplace)
	options, err := s.BeginKey(t.Context(), v, session)
	if err != nil {
		t.Fatal(err)
	}
	key := newSoftKey(t, v.BaseURL)
	codes, err := s.ConfirmKey(t.Context(), v, session, "YubiKey", key.register(options))
	if err != nil {
		t.Fatalf("ConfirmKey = %v", err)
	}
	return session, key, codes
}

// signInWithKey signs in with the password and answers with key.
func signInWithKey(t *testing.T, s *Service, marketplace, email string, key *softKey, answer func([]byte) []byte) (SignedIn, error) {
	t.Helper()
	token := challenged(t, s, marketplace, email)
	options, err := s.KeyOptions(t.Context(), visit(marketplace), token, "")
	if err != nil {
		t.Fatalf("KeyOptions = %v", err)
	}
	return s.CompleteSignIn(t.Context(), visit(marketplace), token, Answer{Method: MethodKey, Key: answer(options)})
}

// A key is added by answering the registration the service started, shows
// the recovery codes as the first app or key does, and then signs in as the
// strongest method.
func TestAddingAKeyAndSigningInWithIt(t *testing.T) {
	s, db, one, _, trail := service(t)
	sealed(t, s)
	session, key, codes := withKey(t, s, db, one, "r@example.test")
	if len(codes) != RecoveryCodeCount {
		t.Fatalf("the first key showed %d recovery codes, want %d", len(codes), RecoveryCodeCount)
	}
	security, err := s.Security(t.Context(), visit(one), session)
	if err != nil || len(security.Factors) != 1 || security.Factors[0].Method != MethodKey || security.Factors[0].Label != "YubiKey" {
		t.Fatalf("Security = %+v, %v", security, err)
	}
	if added := trail.entry(t, "identity.second_factor_added"); string(added.After) != `{"method":"webauthn","user_agent":"test"}` {
		t.Fatalf("second_factor_added recorded as %s", added.After)
	}

	token := challenged(t, s, one, "r@example.test")
	pending, err := s.Pending(t.Context(), visit(one), token, "")
	if err != nil || !slices.Equal(pending.Methods, []Method{MethodKey}) {
		t.Fatalf("Pending = %+v, %v; want the key", pending, err)
	}
	signed, err := signInWithKey(t, s, one, "r@example.test", key, key.assert)
	if err != nil {
		t.Fatalf("signing in with the key = %v", err)
	}
	if _, err := s.Authenticate(t.Context(), one, signed.Session); err != nil {
		t.Fatal(err)
	}
}

// An answer made for another challenge is refused: the ceremony the service
// kept with the challenge is the one the answer must carry.
func TestAKeysAnswerToAnotherChallengeIsRefused(t *testing.T) {
	s, db, one, _, _ := service(t)
	sealed(t, s)
	_, key, _ := withKey(t, s, db, one, "r@example.test")

	first := challenged(t, s, one, "r@example.test")
	stale, err := s.KeyOptions(t.Context(), visit(one), first, "")
	if err != nil {
		t.Fatal(err)
	}
	second := challenged(t, s, one, "r@example.test")
	if _, err := s.KeyOptions(t.Context(), visit(one), second, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CompleteSignIn(t.Context(), visit(one), second, Answer{Method: MethodKey, Key: key.assert(stale)}); !errors.Is(err, ErrCodeWrong) {
		t.Fatalf("an answer to another challenge = %v, want ErrCodeWrong", err)
	}
}

// A key's signature counter that does not move forward is refused, and
// audited as a possible clone (D6).
func TestAKeyWhoseCounterWentBackwardsIsRefused(t *testing.T) {
	s, db, one, _, trail := service(t)
	sealed(t, s)
	_, key, _ := withKey(t, s, db, one, "r@example.test")
	if _, err := signInWithKey(t, s, one, "r@example.test", key, key.assert); err != nil {
		t.Fatal(err)
	}
	// The same counter again: what a copy of the key that has not been
	// used since the copy was made presents.
	if _, err := signInWithKey(t, s, one, "r@example.test", key, key.sign); !errors.Is(err, ErrCodeWrong) {
		t.Fatalf("a counter that did not move = %v, want ErrCodeWrong", err)
	}
	clone := trail.entry(t, "identity.second_factor_clone_suspected")
	if clone.Actor.ID != "" || string(clone.After) != `{"presented":"1","stored":"1","user_agent":"test"}` {
		t.Fatalf("clone_suspected recorded as %+v, %s", clone, clone.After)
	}
}

// A key belongs to the marketplace it was added on: the relying party is the
// marketplace's host, so an answer made for another host is refused.
func TestAKeyAnswersOnlyForItsOwnMarketplacesHost(t *testing.T) {
	s, db, one, _, _ := service(t)
	sealed(t, s)
	_, key, _ := withKey(t, s, db, one, "r@example.test")
	key.origin = "https://elsewhere.test"
	if _, err := signInWithKey(t, s, one, "r@example.test", key, key.assert); !errors.Is(err, ErrCodeWrong) {
		t.Fatalf("an answer made for another host = %v, want ErrCodeWrong", err)
	}
}

// A registration made for another host is refused, and adds nothing.
func TestAKeyRegisteredForAnotherHostIsRefused(t *testing.T) {
	s, db, one, _, _ := service(t)
	sealed(t, s)
	session := signedIn(t, s, db, one, "r@example.test")
	options, err := s.BeginKey(t.Context(), visit(one), session)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ConfirmKey(t.Context(), visit(one), session, "", newSoftKey(t, "https://elsewhere.test").register(options)); !errors.Is(err, ErrKeyRefused) {
		t.Fatalf("a registration for another host = %v, want ErrKeyRefused", err)
	}
	if _, err := s.ConfirmKey(t.Context(), visit(one), session, "", []byte("not json")); !errors.Is(err, ErrKeyRefused) {
		t.Fatalf("a malformed registration = %v, want ErrKeyRefused", err)
	}
	if security, err := s.Security(t.Context(), visit(one), session); err != nil || len(security.Factors) != 0 {
		t.Fatalf("a refused key was added: %+v, %v", security, err)
	}
}
