package identity

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"testing"
)

// A token is looked up by its SHA-256 and never compared itself: the database
// holds only the hash, so there is no secret-dependent comparison to time
// (spec, D6).
func TestTokensAreRandomAndStoredOnlyAsTheirHash(t *testing.T) {
	a, hashA, err := NewToken()
	if err != nil {
		t.Fatal(err)
	}
	b, _, _ := NewToken()
	if a == b {
		t.Fatal("two tokens are equal")
	}
	raw, err := base64.RawURLEncoding.DecodeString(a)
	if err != nil || len(raw) != 32 {
		t.Fatalf("token is %d bytes (%v), want 32", len(raw), err)
	}
	want := sha256.Sum256([]byte(a))
	if !bytes.Equal(hashA, want[:]) || !bytes.Equal(HashToken(a), hashA) {
		t.Fatal("the stored hash is not the SHA-256 of the token")
	}
}
