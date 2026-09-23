package identity

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
)

// NewToken returns a token to hand to a person once, and the hash that is the
// only thing stored (spec, D6). A token is found by the SHA-256 of 32 random
// bytes, so the database never compares the secret itself, and a lookup's
// timing could teach an attacker only about a hash whose preimage they would
// still need.
func NewToken() (string, []byte, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	return token, HashToken(token), nil
}

// HashToken is how a token is looked up.
func HashToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}
