//go:build integration

package identity

import (
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
