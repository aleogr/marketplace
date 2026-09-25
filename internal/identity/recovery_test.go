package identity

import (
	"bytes"
	"regexp"
	"testing"
)

// Ten codes of the form xxxx-xxxx-xxxx, twelve symbols of a 32-letter
// alphabet each: sixty bits, all different (spec, D6).
func TestRecoveryCodesAreTenAndLookAlike(t *testing.T) {
	codes, err := NewRecoveryCodes()
	if err != nil {
		t.Fatal(err)
	}
	if len(codes) != RecoveryCodeCount {
		t.Fatalf("%d codes, want %d", len(codes), RecoveryCodeCount)
	}
	shape := regexp.MustCompile(`^[0-9a-hjkmnp-tv-z]{4}-[0-9a-hjkmnp-tv-z]{4}-[0-9a-hjkmnp-tv-z]{4}$`)
	seen := map[string]bool{}
	for _, code := range codes {
		if !shape.MatchString(code) {
			t.Errorf("%q is not xxxx-xxxx-xxxx in the recovery alphabet", code)
		}
		if seen[code] {
			t.Errorf("%q was issued twice", code)
		}
		seen[code] = true
	}
}

// A code is read the way people copy it: in capitals, without the hyphens,
// with spaces, or with an O for a zero and an I or an L for a one. Each form
// is the same hash; anything else is not a code at all.
func TestARecoveryCodeIsReadAsPeopleCopyIt(t *testing.T) {
	want, ok := recoveryHash("0a1b-2c3d-4e5f")
	if !ok {
		t.Fatal("a well-formed code was refused")
	}
	for _, typed := range []string{"0A1B-2C3D-4E5F", "0a1b2c3d4e5f", " 0a1b 2c3d 4e5f ", "oa1b-2c3d-4e5f", "0aIb-2c3d-4e5f", "0aLb-2c3d-4e5f"} {
		got, ok := recoveryHash(typed)
		if !ok || !bytes.Equal(got, want) {
			t.Errorf("%q does not read as 0a1b-2c3d-4e5f", typed)
		}
	}
	for _, typed := range []string{"", "0a1b-2c3d", "0a1b-2c3d-4e5f-0", "0a1b-2c3d-4e5u", "123456"} {
		if _, ok := recoveryHash(typed); ok {
			t.Errorf("%q was read as a code", typed)
		}
	}
}
