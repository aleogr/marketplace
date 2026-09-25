package identity

import (
	"slices"
	"testing"
)

var (
	everyKind   = []UserKind{KindBuyer, KindStaff, KindStore}
	everyMethod = []Method{MethodKey, MethodApp, MethodEmail}
	everyUse    = []Use{Enrol, Verify}
	everyAction = []Action{ActionPassword, ActionFactors, ActionAddCard, ActionChangeEmail, ActionStore}
)

// Every combination of user kind, method and use, against §18.2: staff never
// with an e-mail code, neither to enrol nor to verify, because e-mail is their
// recovery channel; everybody else with any method.
func TestThePolicyPermitsMethodsAsSection18Point2Says(t *testing.T) {
	var p Policy
	for _, kind := range everyKind {
		for _, method := range everyMethod {
			for _, use := range everyUse {
				want := kind != KindStaff || method != MethodEmail
				if got := p.Permits(kind, method, use); got != want {
					t.Errorf("Permits(%s, %s, %s) = %v, want %v", kind, method, use, got, want)
				}
			}
		}
	}
}

// Staff must have a second factor; buyers and stores may.
func TestOnlyStaffAreRequiredToHaveASecondFactor(t *testing.T) {
	var p Policy
	for kind, want := range map[UserKind]bool{KindBuyer: false, KindStaff: true, KindStore: false} {
		if got := p.Required(kind); got != want {
			t.Errorf("Required(%s) = %v, want %v", kind, got, want)
		}
	}
}

// Every combination of user kind, action and whether the account has a
// second factor. Changing the password or the factors asks again only whoever
// has one (D2), whatever the kind: staff must end up with a factor (F15
// enforces that at sign-in), but is not asked before enrolling the first one,
// or could never enrol one at all. Adding a card, changing the address and a
// store's sensitive actions always ask, with a code to the account's own
// address accepted where the policy permits e-mail (§18.2).
func TestThePolicyAsksForAStepUpAsSection18Point2Says(t *testing.T) {
	var p Policy
	for _, kind := range everyKind {
		for _, action := range everyAction {
			for _, has := range []bool{false, true} {
				want := StepUp{Asked: true, Email: kind != KindStaff}
				if action == ActionPassword || action == ActionFactors {
					want = StepUp{Asked: has}
				}
				if got := p.StepUpFor(kind, action, has); got != want {
					t.Errorf("StepUpFor(%s, %s, has factor %v) = %+v, want %+v", kind, action, has, got, want)
				}
			}
		}
	}
}

// Whatever the policy does not know is refused, never allowed: an unknown
// kind permits nothing and is required a factor, and an unknown action asks.
func TestThePolicyFailsClosed(t *testing.T) {
	var p Policy
	for _, method := range everyMethod {
		if p.Permits("owner?", method, Verify) {
			t.Errorf("an unknown kind may verify with %s", method)
		}
	}
	if !p.Required("owner?") {
		t.Error("an unknown kind is not required a second factor")
	}
	if got := p.StepUpFor(KindBuyer, "delete_account", false); !got.Asked {
		t.Errorf("an unknown action = %+v, want a step-up", got)
	}
	if p.Permits(KindBuyer, "sms", Enrol) {
		t.Error("an unknown method was permitted")
	}
}

// The second step offers the strongest method first: a key, then an app,
// then an e-mail code (spec, Flows).
func TestMethodsAreOfferedStrongestFirst(t *testing.T) {
	got := Strongest([]Method{MethodEmail, MethodApp, MethodKey, MethodApp})
	if want := []Method{MethodKey, MethodApp, MethodEmail}; !slices.Equal(got, want) {
		t.Fatalf("Strongest = %v, want %v", got, want)
	}
}
