package identity

import (
	"encoding/base32"
	"strings"
	"testing"
	"time"
)

// rfcSecret is the SHA-1 key of RFC 6238, appendix B: the ASCII of
// "12345678901234567890".
var rfcSecret = []byte("12345678901234567890")

// The test vectors of RFC 6238, appendix B, for HMAC-SHA-1. They are eight
// digits; the service's six are the same value's last six, and are checked
// too.
func TestTOTPMatchesTheRFC6238Vectors(t *testing.T) {
	for _, v := range []struct {
		unix  int64
		eight string
	}{
		{59, "94287082"},
		{1111111109, "07081804"},
		{1111111111, "14050471"},
		{1234567890, "89005924"},
		{2000000000, "69279037"},
		{20000000000, "65353130"},
	} {
		step := totpStep(time.Unix(v.unix, 0))
		if got := hotp(rfcSecret, step, 8); got != v.eight {
			t.Errorf("at %d: hotp(8 digits) = %s, want %s", v.unix, got, v.eight)
		}
		if got := totpCode(rfcSecret, step); got != v.eight[2:] {
			t.Errorf("at %d: totpCode = %s, want %s", v.unix, got, v.eight[2:])
		}
	}
}

// A code is accepted for the previous, the current and the next step, and
// for no step further away (spec, D6).
func TestTOTPAcceptsOneStepOfDriftEitherWay(t *testing.T) {
	now := time.Unix(1111111111, 0)
	current := totpStep(now)
	for offset := int64(-1); offset <= 1; offset++ {
		code := totpCode(rfcSecret, current+offset)
		step, ok := matchTOTP(rfcSecret, code, now, 0)
		if !ok || step != current+offset {
			t.Errorf("the code of step %+d = step %d, %v; want step %d accepted", offset, step, ok, current+offset)
		}
	}
	for _, offset := range []int64{-2, 2} {
		if _, ok := matchTOTP(rfcSecret, totpCode(rfcSecret, current+offset), now, 0); ok {
			t.Errorf("the code of step %+d was accepted", offset)
		}
	}
}

// A code is never accepted for a step at or before the last one accepted: a
// code seen once, over a shoulder or in a log, is spent (spec, D6).
func TestTOTPRefusesAReplayedCode(t *testing.T) {
	now := time.Unix(1234567890, 0)
	code := totpCode(rfcSecret, totpStep(now))
	step, ok := matchTOTP(rfcSecret, code, now, 0)
	if !ok {
		t.Fatal("a current code was refused")
	}
	if _, ok := matchTOTP(rfcSecret, code, now, step); ok {
		t.Fatal("the same code was accepted twice")
	}
	// Nor is the previous step's code, once a later one was accepted.
	if _, ok := matchTOTP(rfcSecret, totpCode(rfcSecret, step-1), now, step); ok {
		t.Fatal("an older code was accepted after a newer one")
	}
	// The next step's code is still good.
	if next, ok := matchTOTP(rfcSecret, totpCode(rfcSecret, step+1), now, step); !ok || next != step+1 {
		t.Fatalf("the next step's code = %d, %v", next, ok)
	}
}

// What a person types is read the way it is shown: spaces are ignored, and
// anything that is not six digits is refused without being compared.
func TestTOTPReadsTheCodeAsTyped(t *testing.T) {
	now := time.Unix(2000000000, 0)
	code := totpCode(rfcSecret, totpStep(now))
	if _, ok := matchTOTP(rfcSecret, code[:3]+" "+code[3:], now, 0); !ok {
		t.Error("a code typed with a space was refused")
	}
	for _, typed := range []string{"", "12345", "1234567", "abcdef", code + "0"} {
		if _, ok := matchTOTP(rfcSecret, typed, now, 0); ok {
			t.Errorf("%q was accepted", typed)
		}
	}
}

func TestATOTPSecretIs160RandomBits(t *testing.T) {
	a, err := NewTOTPSecret()
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewTOTPSecret()
	if err != nil {
		t.Fatal(err)
	}
	if len(a) != 20 || string(a) == string(b) {
		t.Fatalf("secrets %x and %x: want 20 bytes each, different", a, b)
	}
	// Base32 without padding: 32 characters for 20 bytes, which is what an
	// app asks a person to type.
	if key := TOTPKey(a); len(key) != 32 || strings.Contains(key, "=") {
		t.Fatalf("TOTPKey = %q", key)
	}
}

// The URI the QR code carries names the marketplace and the account, and the
// parameters every app reads; a space is written %20, which every app decodes,
// and never +, which some show as it is.
func TestTheTOTPURICarriesTheIssuerAndTheParameters(t *testing.T) {
	uri := TOTPURI("Loja Um", "leitora@example.test", rfcSecret)
	// The key is built rather than written out: the RFC's published test
	// seed is not a secret, but a literal key after "secret=" reads like one
	// to the secret scanner (gitleaks) that CI runs.
	key := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(rfcSecret)
	want := "otpauth://totp/Loja%20Um:leitora%40example.test?secret=" + key +
		"&issuer=Loja%20Um&algorithm=SHA1&digits=6&period=30"
	if uri != want {
		t.Fatalf("TOTPURI =\n %s\nwant\n %s", uri, want)
	}
}
