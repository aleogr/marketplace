package identity

import "slices"

// UserKind is whose account it is, as the second-factor policy reads it
// (docs/requirements.md, section 18.2).
type UserKind string

const (
	KindBuyer UserKind = "buyer"
	KindStaff UserKind = "staff"
	// KindStore is a store acting in its own store. Stores arrive in a later
	// phase; the policy is theirs already, so nothing about it is decided
	// then.
	KindStore UserKind = "store"
)

// Method is a kind of second factor, as second_factor.kind stores it.
type Method string

const (
	// MethodKey is a security key or the device's own authenticator
	// (WebAuthn; spec, D1).
	MethodKey Method = "webauthn"
	// MethodApp is an authenticator app (TOTP).
	MethodApp Method = "totp"
	// MethodEmail is a code sent to the account's address.
	MethodEmail Method = "email"
)

// Use is what a method is being used for.
type Use string

const (
	// Enrol is adding the method to an account.
	Enrol Use = "enrol"
	// Verify is answering with it, at sign-in or at a step-up.
	Verify Use = "verify"
)

// Action is something sensitive enough to ask for a second factor again.
type Action string

const (
	ActionPassword    Action = "password"
	ActionFactors     Action = "factors"
	ActionAddCard     Action = "add_card"
	ActionChangeEmail Action = "change_email"
	// ActionStore is a store's sensitive action: a bank or payout account
	// change, bulk label generation, the acceptance of terms (§18.2).
	ActionStore Action = "store"
)

// StepUp is what an action asks of a session.
type StepUp struct {
	// Asked is whether the action needs a recent step-up (D4).
	Asked bool
	// Email is whether a code sent to the account's own address answers the
	// step-up even when the account enrolled no e-mail factor: §18.2's
	// default for a buyer adding a card or changing the address, and for a
	// new store.
	Email bool
}

// Policy is §18.2 as a service, and the one place every path asks: which
// methods a kind of user may enrol and answer with, whether they must have a
// second factor, and what a sensitive action asks of them. It holds no state,
// and whatever it does not know it refuses.
type Policy struct{}

// Permits reports whether kind may use method for use. Staff may never use an
// e-mail code, neither to enrol nor to verify: e-mail is their recovery
// channel, and a factor that recovery can reset is not a second factor.
func (Policy) Permits(kind UserKind, method Method, use Use) bool {
	if !slices.Contains([]Method{MethodKey, MethodApp, MethodEmail}, method) || (use != Enrol && use != Verify) {
		return false
	}
	switch kind {
	case KindBuyer, KindStore:
		return true
	case KindStaff:
		return method != MethodEmail
	}
	return false
}

// Required reports whether an account of kind must have a second factor:
// staff must (it takes effect when staff sign in, F15), buyers and stores may.
// An unknown kind must.
func (Policy) Required(kind UserKind) bool {
	return kind != KindBuyer && kind != KindStore
}

// StepUpFor is what action asks of an account of kind, which has or has not
// a second factor. Changing the password or the factors asks again whoever
// has one (D2), whatever the kind: staff must end up with a factor (Required,
// enforced at sign-in by F15), but is not asked before enrolling the first
// one, or could never enrol one at all. A kind this policy does not know is
// asked even without one: it fails closed. Everything else, including an
// action this policy does not know, always asks, and accepts a code to the
// account's own address where the kind may use e-mail at all.
func (p Policy) StepUpFor(kind UserKind, action Action, hasFactor bool) StepUp {
	switch action {
	case ActionPassword, ActionFactors:
		return StepUp{Asked: hasFactor || (p.Required(kind) && kind != KindStaff)}
	}
	return StepUp{Asked: true, Email: p.Permits(kind, MethodEmail, Verify)}
}

// strength orders the methods the second step offers, strongest first: a key
// cannot be phished, an app's code can, and an e-mail code is only as strong
// as the mailbox (spec, Flows).
var strength = []Method{MethodKey, MethodApp, MethodEmail}

// Strongest returns the methods given, each once, strongest first.
func Strongest(methods []Method) []Method {
	var ordered []Method
	for _, method := range strength {
		if slices.Contains(methods, method) {
			ordered = append(ordered, method)
		}
	}
	return ordered
}
