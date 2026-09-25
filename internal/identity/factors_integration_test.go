//go:build integration

package identity

import (
	"bytes"
	"errors"
	"slices"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/aleogr/marketplace/internal/platform/audit"
	"github.com/aleogr/marketplace/internal/platform/keys"
	"github.com/aleogr/marketplace/internal/platform/keys/local"
)

// sealed gives a service the audit log's per-person keys, wrapped by a
// keeper of the test's own, and returns them so a test can destroy one.
func sealed(t *testing.T, s *Service) *audit.Keys {
	t.Helper()
	keeper, err := local.Generate()
	if err != nil {
		t.Fatal(err)
	}
	personal := audit.NewKeys(keeper)
	s.WithSealer(personal)
	return personal
}

// signedIn confirms an account, signs it in and returns its session.
func signedIn(t *testing.T, s *Service, db serving, marketplace, email string) Session {
	t.Helper()
	confirmed(t, s, db, marketplace, email)
	session, err := s.Authenticate(t.Context(), marketplace, signIn(t, s, marketplace, email, "correct horse battery staple"))
	if err != nil {
		t.Fatal(err)
	}
	return session
}

// appCode is the code an app set up with enrolment shows now.
func appCode(t *testing.T, s *Service, enrolment AppEnrolment) string {
	t.Helper()
	secret, err := totpEncoding.DecodeString(enrolment.Key)
	if err != nil {
		t.Fatal(err)
	}
	return totpCode(secret, totpStep(s.now()))
}

// enrolApp adds an app to the session's account and returns the recovery
// codes the enrolment showed, if it showed any.
func enrolApp(t *testing.T, s *Service, v Visit, session Session, label string) (AppEnrolment, []string) {
	t.Helper()
	enrolment, err := s.BeginApp(t.Context(), v, session)
	if err != nil {
		t.Fatal(err)
	}
	codes, err := s.ConfirmApp(t.Context(), v, session, label, appCode(t, s, enrolment))
	if err != nil {
		t.Fatalf("ConfirmApp = %v", err)
	}
	return enrolment, codes
}

// An app becomes a second factor only when a code it made is typed; the
// first one shows the ten recovery codes, once.
func TestAddingAnAppProvesItAndIssuesTheRecoveryCodes(t *testing.T) {
	s, db, one, _, trail := service(t)
	sealed(t, s)
	session := signedIn(t, s, db, one, "r@example.test")
	v := visit(one)

	enrolment, err := s.BeginApp(t.Context(), v, session)
	if err != nil {
		t.Fatal(err)
	}
	if again, err := s.PendingApp(t.Context(), v, session); err != nil || again != enrolment {
		t.Fatalf("PendingApp = %+v, %v; want the enrolment BeginApp returned", again, err)
	}
	if _, err := s.ConfirmApp(t.Context(), v, session, "phone", "000000"); !errors.Is(err, ErrCodeWrong) {
		t.Fatalf("a wrong code = %v, want ErrCodeWrong", err)
	}
	if security, err := s.Security(t.Context(), v, session); err != nil || len(security.Factors) != 0 {
		t.Fatalf("after a wrong code the account has %+v, %v; want no factor", security, err)
	}

	codes, err := s.ConfirmApp(t.Context(), v, session, " phone ", appCode(t, s, enrolment))
	if err != nil {
		t.Fatalf("the right code = %v", err)
	}
	if len(codes) != RecoveryCodeCount {
		t.Fatalf("the first app showed %d recovery codes, want %d", len(codes), RecoveryCodeCount)
	}
	security, err := s.Security(t.Context(), v, session)
	if err != nil {
		t.Fatal(err)
	}
	if len(security.Factors) != 1 || security.Factors[0].Method != MethodApp || security.Factors[0].Label != "phone" ||
		security.RecoveryIssued != 10 || security.RecoveryLeft != 10 {
		t.Fatalf("Security = %+v", security)
	}
	if _, err := s.PendingApp(t.Context(), v, session); !errors.Is(err, ErrNoEnrolment) {
		t.Fatalf("the enrolment survived its confirmation: %v", err)
	}

	// A second app shows no codes: the account already has its set.
	if _, more := enrolApp(t, s, v, session, "tablet"); more != nil {
		t.Fatalf("the second app showed recovery codes %v", more)
	}
	added := trail.entry(t, "identity.second_factor_added")
	if added.Actor.ID != session.Account.ID || added.Subject.Person != session.Account.ID ||
		!slices.Contains(trail.actions(), "identity.second_factor_added") {
		t.Fatalf("second_factor_added recorded as %+v", added)
	}
}

// Removing the last method turns 2FA off and deletes the recovery codes;
// removing one of two keeps both the other method and the codes.
func TestRemovingTheLastMethodTurnsTwoFactorOff(t *testing.T) {
	s, db, one, _, trail := service(t)
	sealed(t, s)
	session := signedIn(t, s, db, one, "r@example.test")
	v := visit(one)
	enrolApp(t, s, v, session, "phone")
	enrolApp(t, s, v, session, "tablet")

	security, err := s.Security(t.Context(), v, session)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.RemoveFactor(t.Context(), v, session, security.Factors[0].ID); err != nil {
		t.Fatal(err)
	}
	if security, err = s.Security(t.Context(), v, session); err != nil || len(security.Factors) != 1 || security.RecoveryIssued != 10 {
		t.Fatalf("after removing one of two: %+v, %v", security, err)
	}
	last := security.Factors[0].ID
	if err := s.RemoveFactor(t.Context(), v, session, last); err != nil {
		t.Fatal(err)
	}
	if security, err = s.Security(t.Context(), v, session); err != nil || len(security.Factors) != 0 || security.RecoveryIssued != 0 {
		t.Fatalf("after removing the last: %+v, %v; want no factor and no recovery code", security, err)
	}
	if removed := trail.entry(t, "identity.second_factor_removed"); removed.Actor.ID != session.Account.ID {
		t.Fatalf("second_factor_removed recorded as %+v", removed)
	}
	if err := s.RemoveFactor(t.Context(), v, session, last); !errors.Is(err, ErrFactorUnknown) {
		t.Fatalf("removing a factor that is gone = %v, want ErrFactorUnknown", err)
	}
}

// One account cannot remove another's factor: the factor is looked up
// within the session's own account.
func TestAnotherAccountsFactorIsUnknown(t *testing.T) {
	s, db, one, _, _ := service(t)
	sealed(t, s)
	v := visit(one)
	owner := signedIn(t, s, db, one, "owner@example.test")
	stranger := signedIn(t, s, db, one, "stranger@example.test")
	enrolApp(t, s, v, owner, "phone")
	security, err := s.Security(t.Context(), v, owner)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.RemoveFactor(t.Context(), v, stranger, security.Factors[0].ID); !errors.Is(err, ErrFactorUnknown) {
		t.Fatalf("another account removing the factor = %v, want ErrFactorUnknown", err)
	}
}

// Staff must keep a second factor (§18.2): the last one cannot be removed,
// and nothing changes when that is refused.
func TestStaffCannotRemoveTheirLastFactor(t *testing.T) {
	s, db, one, _, _ := service(t)
	sealed(t, s)
	v := visit(one)
	session := signedIn(t, s, db, one, "r@example.test")
	enrolApp(t, s, v, session, "phone")
	security, err := s.Security(t.Context(), v, session)
	if err != nil {
		t.Fatal(err)
	}

	// Staff always step up before changing their factors; this one just did.
	staff := session
	staff.Account.Kind = KindStaff
	now := s.now()
	staff.SteppedUpAt = &now
	if err := s.RemoveFactor(t.Context(), v, staff, security.Factors[0].ID); !errors.Is(err, ErrFactorRequired) {
		t.Fatalf("staff removing their last factor = %v, want ErrFactorRequired", err)
	}
	if after, err := s.Security(t.Context(), v, session); err != nil || len(after.Factors) != 1 || after.RecoveryIssued != 10 {
		t.Fatalf("the refused removal changed the account: %+v, %v", after, err)
	}
}

// New recovery codes replace the old ones, which stop working; an account
// with no second factor has none to regenerate.
func TestRegeneratingRecoveryCodesInvalidatesTheOldOnes(t *testing.T) {
	s, db, one, _, trail := service(t)
	sealed(t, s)
	v := visit(one)
	session := signedIn(t, s, db, one, "r@example.test")
	if _, err := s.RegenerateRecoveryCodes(t.Context(), v, session); !errors.Is(err, ErrNoSecondFactor) {
		t.Fatalf("regenerating with no factor = %v, want ErrNoSecondFactor", err)
	}
	_, old := enrolApp(t, s, v, session, "phone")
	fresh, err := s.RegenerateRecoveryCodes(t.Context(), v, session)
	if err != nil || len(fresh) != RecoveryCodeCount {
		t.Fatalf("RegenerateRecoveryCodes = %d codes, %v", len(fresh), err)
	}
	stored := storedRecoveryHashes(t, db, one, session.Account.ID)
	oldHash := boundRecoveryHash(session.Account.ID, func() []byte { h, _ := recoveryHash(old[0]); return h }())
	freshHash := boundRecoveryHash(session.Account.ID, func() []byte { h, _ := recoveryHash(fresh[0]); return h }())
	if len(stored) != RecoveryCodeCount || slices.ContainsFunc(stored, func(h []byte) bool { return bytes.Equal(h, oldHash) }) ||
		!slices.ContainsFunc(stored, func(h []byte) bool { return bytes.Equal(h, freshHash) }) {
		t.Fatal("the stored codes are not exactly the new set")
	}
	trail.entry(t, "identity.recovery_codes_regenerated")
}

// storedRecoveryHashes are the hashes of an account's recovery codes.
func storedRecoveryHashes(t *testing.T, db serving, marketplace, account string) [][]byte {
	t.Helper()
	var hashes [][]byte
	if err := db.InTxFor(t.Context(), marketplace, func(tx pgx.Tx) error {
		rows, err := tx.Query(t.Context(), `SELECT code_hash FROM recovery_code WHERE account_id = $1`, account)
		if err != nil {
			return err
		}
		hashes, err = pgx.CollectRows(rows, pgx.RowTo[[]byte])
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return hashes
}

// The app's secret is sealed with the person's own key: once that key is
// destroyed for an erasure request, nothing can read the secret (D5).
func TestTheAppSecretIsUnreadableOnceThePersonsKeyIsDestroyed(t *testing.T) {
	s, db, one, _, _ := service(t)
	personal := sealed(t, s)
	v := visit(one)
	session := signedIn(t, s, db, one, "r@example.test")
	enrolment, _ := enrolApp(t, s, v, session, "phone")

	open := func() ([]byte, error) {
		var secret []byte
		err := db.InTxFor(t.Context(), one, func(tx pgx.Tx) error {
			var stored []byte
			if err := tx.QueryRow(t.Context(), `SELECT secret FROM second_factor WHERE account_id = $1`,
				session.Account.ID).Scan(&stored); err != nil {
				return err
			}
			if string(stored) == enrolment.Key {
				t.Error("the secret is stored as it is shown")
			}
			var err error
			secret, err = personal.OpenFor(t.Context(), tx, session.Account.ID, stored)
			return err
		})
		return secret, err
	}
	if secret, err := open(); err != nil || TOTPKey(secret) != enrolment.Key {
		t.Fatalf("the stored secret opens as %q, %v; want the enrolled key", TOTPKey(secret), err)
	}
	if err := db.InTxFor(t.Context(), one, func(tx pgx.Tx) error {
		return personal.Destroy(t.Context(), tx, session.Account.ID)
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := open(); !errors.Is(err, keys.ErrDestroyed) {
		t.Fatalf("the secret after the key was destroyed = %v, want keys.ErrDestroyed", err)
	}
}
