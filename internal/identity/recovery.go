package identity

import (
	"crypto/rand"
	"crypto/sha256"
	"strings"
)

// RecoveryCodeCount is how many recovery codes an account holds at once
// (spec, D6).
const RecoveryCodeCount = 10

// recoveryAlphabet is Crockford's base32 in lower case: no i, l, o or u, so a
// code copied by hand cannot be misread. Thirty-two symbols are five bits
// each, and twelve of them are sixty bits: too many to guess online. Across a
// stolen database that stored every account's hashes the same way, sixty
// bits is a multi-target search rather than a per-account one, so the stored
// value is bound to the account (boundRecoveryHash) instead of being the
// plain hash of the code (owner's decision, 2026-09-25).
const (
	recoveryAlphabet = "0123456789abcdefghjkmnpqrstvwxyz"
	recoverySymbols  = 12
)

// NewRecoveryCodes returns a fresh set of codes, as the person is shown them
// once: xxxx-xxxx-xxxx.
func NewRecoveryCodes() ([]string, error) {
	codes := make([]string, RecoveryCodeCount)
	raw := make([]byte, recoverySymbols)
	for i := range codes {
		if _, err := rand.Read(raw); err != nil {
			return nil, err
		}
		var code strings.Builder
		for j, b := range raw {
			if j > 0 && j%4 == 0 {
				code.WriteByte('-')
			}
			// 256 is a multiple of 32, so the low five bits of a random byte
			// are an unbiased symbol.
			code.WriteByte(recoveryAlphabet[b&31])
		}
		codes[i] = code.String()
	}
	return codes, nil
}

// recoveryHash is how a code is stored and looked up: the SHA-256 of its
// twelve symbols, read the way people copy them (any case, hyphens and spaces
// ignored, O read as zero and I or L as one). It reports false for anything
// that is not a code, which is then not compared with anything.
func recoveryHash(typed string) ([]byte, bool) {
	symbols := strings.Map(func(r rune) rune {
		switch r {
		case '-', ' ':
			return -1
		case 'o':
			return '0'
		case 'i', 'l':
			return '1'
		}
		return r
	}, strings.ToLower(typed))
	if len(symbols) != recoverySymbols || strings.Trim(symbols, recoveryAlphabet) != "" {
		return nil, false
	}
	sum := sha256.Sum256([]byte(symbols))
	return sum[:], true
}

// boundRecoveryHash ties a code's hash to the account it belongs to: the
// SHA-256 of the account id's bytes followed by hash. account is always a
// fixed-length UUID string, so the concatenation of account and hash is
// unambiguous; there is no delimiter to confuse with either part. This is
// how recovery codes are stored (replaceRecoveryCodes) and looked up, so
// that a stolen database of hashes must be searched one account at a time
// rather than once for every account (owner's decision, 2026-09-25).
func boundRecoveryHash(account string, hash []byte) []byte {
	h := sha256.New()
	h.Write([]byte(account))
	h.Write(hash)
	return h.Sum(nil)
}
