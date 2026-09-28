//go:build integration

package identity

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/aleogr/marketplace/internal/platform/audit"
)

// platformVisit is a request on the console's host: a staff member's, which
// belongs to no marketplace (F15 spec, D4).
func platformVisit() Visit {
	return Visit{Platform: true, MarketplaceName: "Console", Language: "en-US",
		IP: "203.0.113.9", UserAgent: "test", BaseURL: "https://console.test"}
}

// staffMember creates a staff account on the platform, with nothing run in
// its transaction, and returns its address and the token of the session it
// opened.
func staffMember(t *testing.T, s *Service) (string, string) {
	t.Helper()
	email := unique(t, "staff") + "@example.test"
	token, err := s.CreateStaff(t.Context(), platformVisit(), "Staff Member", email, "correct horse battery staple", nil)
	if err != nil {
		t.Fatalf("CreateStaff = %v", err)
	}
	return email, token
}

// A staff account is created on the platform, signed in there, and exists
// nowhere else: the session it opened is the platform's, and no marketplace
// can open it or sign the account in.
func TestAStaffAccountBelongsToThePlatform(t *testing.T) {
	s, _, one, _, trail := service(t)
	email, token := staffMember(t, s)

	session, err := s.AuthenticateStaff(t.Context(), token)
	if err != nil {
		t.Fatalf("AuthenticateStaff = %v", err)
	}
	if session.Account.Kind != KindStaff || session.Account.MarketplaceID != "" || session.Account.Name != "Staff Member" ||
		session.Account.VerifiedAt == nil || session.SecondFactor {
		t.Fatalf("the staff session is %+v", session)
	}
	if _, err := s.Authenticate(t.Context(), one, token); !errors.Is(err, ErrSessionInvalid) {
		t.Fatalf("a marketplace opened a staff session: %v", err)
	}

	// Signing in again: on the platform it opens a session (no factor yet),
	// and in a marketplace the address is unknown.
	again, err := s.SignIn(t.Context(), platformVisit(), email, "correct horse battery staple")
	if err != nil || again == "" {
		t.Fatalf("SignIn on the platform = %q, %v", again, err)
	}
	if _, err := s.SignIn(t.Context(), visit(one), email, "correct horse battery staple"); !errors.Is(err, ErrCredentials) {
		t.Fatalf("SignIn of a staff member in a marketplace = %v, want ErrCredentials", err)
	}

	created := trail.entry(t, "identity.staff_created")
	if created.Marketplace != "" || created.Actor.Kind != audit.Staff || created.Actor.ID != session.Account.ID {
		t.Fatalf("identity.staff_created recorded as %+v", created)
	}
	if signin := trail.entry(t, "identity.signin"); signin.Actor.Kind != audit.Staff || signin.Marketplace != "" {
		t.Fatalf("a staff sign-in recorded as %+v", signin)
	}
}

// Nor is a buyer known on the platform: the address of a marketplace's
// account signs in to nothing there, and its sessions do not open.
func TestABuyerIsUnknownOnThePlatform(t *testing.T) {
	s, db, one, _, _ := service(t)
	confirmed(t, s, db, one, "buyer@example.test")
	token := signIn(t, s, one, "buyer@example.test", "correct horse battery staple")

	if _, err := s.SignIn(t.Context(), platformVisit(), "buyer@example.test", "correct horse battery staple"); !errors.Is(err, ErrCredentials) {
		t.Fatalf("a buyer signing in on the platform = %v, want ErrCredentials", err)
	}
	if _, err := s.AuthenticateStaff(t.Context(), token); !errors.Is(err, ErrSessionInvalid) {
		t.Fatalf("the platform opened a buyer's session: %v", err)
	}
}

// Staff reuse F14 whole: an app added on the platform, with its recovery
// codes, then a sign-in that asks for it; and never an e-mail code.
func TestStaffSignInInTwoStepsOnThePlatform(t *testing.T) {
	s, _, _, _, _ := service(t)
	sealed(t, s)
	email, token := staffMember(t, s)
	session, err := s.AuthenticateStaff(t.Context(), token)
	if err != nil {
		t.Fatal(err)
	}
	// A staff member with no factor is not asked to step up before the
	// first one (F14, D2), or could never add one.
	enrolment, codes := enrolApp(t, s, platformVisit(), session, "phone")
	if len(codes) != RecoveryCodeCount {
		t.Fatalf("the first app showed %d recovery codes, want %d", len(codes), RecoveryCodeCount)
	}

	challenge, err := s.SignIn(t.Context(), platformVisit(), email, "correct horse battery staple")
	if !errors.Is(err, ErrSecondStep) {
		t.Fatalf("SignIn with an app = %v, want ErrSecondStep", err)
	}
	pending, err := s.Pending(t.Context(), platformVisit(), challenge, "")
	if err != nil || !slices.Equal(pending.Methods, []Method{MethodApp}) || !pending.Recovery {
		t.Fatalf("Pending = %+v, %v; want the app and the recovery codes, and no e-mail", pending, err)
	}
	if err := s.SendChallengeCode(t.Context(), platformVisit(), challenge, ""); !errors.Is(err, ErrNotPermitted) {
		t.Fatalf("an e-mail code for staff = %v, want ErrNotPermitted", err)
	}
	signed, err := s.CompleteSignIn(t.Context(), platformVisit(), challenge,
		Answer{Method: MethodApp, Code: nextAppCode(t, s, enrolment)})
	if err != nil {
		t.Fatalf("CompleteSignIn = %v", err)
	}
	stepped, err := s.AuthenticateStaff(t.Context(), signed.Session)
	if err != nil || !stepped.SecondFactor || stepped.SteppedUpAt == nil {
		t.Fatalf("the completed staff session is %+v, %v", stepped, err)
	}
}

// What CreateStaff is given to run — the bootstrap's claim of the owner
// role, an invitation's use — runs in the account's own transaction: when it
// fails, no account is left behind.
func TestCreateStaffRunsItsHookInTheSameTransaction(t *testing.T) {
	s, db, _, _, _ := service(t)
	email := unique(t, "staff") + "@example.test"
	refused := errors.New("refused")
	var seen string
	_, err := s.CreateStaff(t.Context(), platformVisit(), "Staff", email, "correct horse battery staple",
		func(ctx context.Context, tx pgx.Tx, account string) error {
			seen = account
			return refused
		})
	if !errors.Is(err, refused) || seen == "" {
		t.Fatalf("CreateStaff with a failing hook = %v (hook saw %q), want the hook's error", err, seen)
	}
	var left int
	if err := db.InTx(t.Context(), func(tx pgx.Tx) error {
		return tx.QueryRow(t.Context(), `SELECT count(*) FROM account WHERE id = $1`, seen).Scan(&left)
	}); err != nil {
		t.Fatal(err)
	}
	if left != 0 {
		t.Fatal("the account of a failed hook was left behind")
	}

	// And once created, the address is taken.
	if _, err := s.CreateStaff(t.Context(), platformVisit(), "Staff", email, "correct horse battery staple", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateStaff(t.Context(), platformVisit(), "Other", email, "another long passphrase", nil); !errors.Is(err, ErrAddressTaken) {
		t.Fatalf("a second staff account at the address = %v, want ErrAddressTaken", err)
	}
}

// F13's rules hold for staff: the name, the address and the password,
// breach check included, before anything is written.
func TestCreateStaffAppliesThePasswordRules(t *testing.T) {
	s, _, _, _, _ := service(t)
	for name, tc := range map[string]struct {
		name, email, password string
		want                  error
	}{
		"a short password":    {"Staff", "a@example.test", "short", ErrPasswordShort},
		"a breached password": {"Staff", "b@example.test", "password1234", ErrPasswordBreached},
		"no name":             {" ", "c@example.test", "correct horse battery staple", ErrNameMissing},
		"no address":          {"Staff", "not an address", "correct horse battery staple", ErrEmailInvalid},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := s.CreateStaff(t.Context(), platformVisit(), tc.name, tc.email, tc.password, nil); !errors.Is(err, tc.want) {
				t.Fatalf("CreateStaff = %v, want %v", err, tc.want)
			}
		})
	}
}

// A platform visit's transactions name no marketplace; one that also names
// a marketplace is a programming error, refused before anything runs.
func TestAPlatformVisitNamesNoMarketplace(t *testing.T) {
	s, _, one, _, _ := service(t)
	v := platformVisit()
	v.Marketplace = one
	if _, err := s.SignIn(t.Context(), v, "someone@example.test", "correct horse battery staple"); err == nil ||
		errors.Is(err, ErrCredentials) {
		t.Fatalf("SignIn on a visit naming both = %v, want a refusal of the visit", err)
	}
}
