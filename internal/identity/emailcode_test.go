package identity

import (
	"bytes"
	"testing"
)

// A code is six digits, and what is stored is its SHA-256 (D5, D6).
func TestAnEmailCodeIsSixDigitsStoredAsItsHash(t *testing.T) {
	seen := map[string]bool{}
	for range 20 {
		code, hash, err := newEmailCode()
		if err != nil {
			t.Fatal(err)
		}
		again, ok := emailCodeHash(code)
		if len(code) != 6 || !ok || !bytes.Equal(hash, again) || bytes.Contains(hash, []byte(code)) {
			t.Fatalf("code %q, hash %x", code, hash)
		}
		seen[code] = true
	}
	if len(seen) < 15 {
		t.Fatalf("twenty codes held only %d different values", len(seen))
	}
}

// A typed code is read with its spaces ignored; anything else that is not
// six digits is not a code.
func TestAnEmailCodeIsReadAsTyped(t *testing.T) {
	want, _ := emailCodeHash("123456")
	if got, ok := emailCodeHash(" 123 456 "); !ok || !bytes.Equal(got, want) {
		t.Error("a code typed with spaces was not read")
	}
	for _, typed := range []string{"", "12345", "1234567", "12345a"} {
		if _, ok := emailCodeHash(typed); ok {
			t.Errorf("%q was read as a code", typed)
		}
	}
}
