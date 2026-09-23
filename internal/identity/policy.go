// Package identity holds accounts, credentials and sessions (docs/design.md,
// section 2.1; docs/superpowers/specs/2026-09-23-f13-identity-core-design.md).
package identity

import (
	"errors"
	"net/mail"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// The password rule is a length and nothing else (spec, D4). NIST SP 800-63B
// says a verifier shall not demand digits, capitals or symbols, because such
// rules produce predictable passwords that guessing tools try first. The
// minimum is the owner's choice of 2026-09-23 and becomes a parameter in F17;
// the pages read both bounds from here, never from a literal of their own.
const (
	MinPasswordLength = 12
	MaxPasswordLength = 128
)

var (
	ErrPasswordShort    = errors.New("identity: password too short")
	ErrPasswordLong     = errors.New("identity: password too long")
	ErrPasswordBreached = errors.New("identity: password found in a breach")
	ErrEmailInvalid     = errors.New("identity: e-mail address invalid")
	ErrNameMissing      = errors.New("identity: name missing")
)

// normalise is what a password is before it is counted or hashed: NFKC, so
// the same password typed on two keyboards is the same password (NIST
// SP 800-63B, section 5.1.1.2).
func normalise(password string) string { return norm.NFKC.String(password) }

// CheckPassword reports whether a password may be set.
func CheckPassword(password string) error {
	n := utf8.RuneCountInString(normalise(password))
	switch {
	case n < MinPasswordLength:
		return ErrPasswordShort
	case n > MaxPasswordLength:
		return ErrPasswordLong
	}
	return nil
}

// NormaliseEmail returns the form an address is compared in, or
// ErrEmailInvalid. Only a bare address is accepted: "Name <a@b>" is valid mail
// syntax and not something a person types into an e-mail field.
func NormaliseEmail(address string) (string, error) {
	address = strings.TrimSpace(address)
	parsed, err := mail.ParseAddress(address)
	if err != nil || parsed.Address != address || parsed.Name != "" {
		return "", ErrEmailInvalid
	}
	return strings.ToLower(address), nil
}
