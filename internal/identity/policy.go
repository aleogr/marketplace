// Package identity holds accounts, credentials and sessions (docs/design.md,
// section 2.1; docs/superpowers/specs/2026-09-23-f13-identity-core-design.md).
package identity

import (
	"errors"
	"net/mail"
	"strings"
	"unicode"
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

// maxPasswordBytes is the longest password, in bytes as typed, that sign-in
// looks at: anything longer is refused before it is normalised or hashed, so a
// huge input costs nothing. It admits every password that can be set. A code
// point is at most four bytes, and NFKC composes at most four code points into
// one (U+1F82 is four), so no password within MaxPasswordLength after
// normalisation is longer than this before it.
const maxPasswordBytes = 16 * MaxPasswordLength

// MaxNameLength bounds the name a person gives, in Unicode code points after
// trimming. The name is stored and written into mail sent to an address the
// sender chose, so it is bounded and holds only visible text; the page reads
// the bound from here, like the password's.
const MaxNameLength = 100

// MaxEmailLength is the longest address mail can carry: RFC 5321 bounds a
// forward path at 256 octets, and two of them are its angle brackets.
const MaxEmailLength = 254

var (
	ErrPasswordShort    = errors.New("identity: password too short")
	ErrPasswordLong     = errors.New("identity: password too long")
	ErrPasswordBreached = errors.New("identity: password found in a breach")
	ErrEmailInvalid     = errors.New("identity: e-mail address invalid")
	ErrNameMissing      = errors.New("identity: name missing")
	ErrNameInvalid      = errors.New("identity: name too long or not plain text")
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

// CheckName returns the name a person gave, trimmed, or why it cannot be kept:
// ErrNameMissing when nothing is left, ErrNameInvalid when it is longer than
// MaxNameLength or holds a control or format character. Control characters
// would break lines in a mail's text; format characters include the
// bidirectional overrides that make text read as something it is not.
func CheckName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", ErrNameMissing
	}
	if !utf8.ValidString(name) || utf8.RuneCountInString(name) > MaxNameLength {
		return "", ErrNameInvalid
	}
	for _, r := range name {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return "", ErrNameInvalid
		}
	}
	return name, nil
}

// NormaliseEmail returns the form an address is compared in, or
// ErrEmailInvalid. Only a bare address is accepted: "Name <a@b>" is valid mail
// syntax and not something a person types into an e-mail field.
func NormaliseEmail(address string) (string, error) {
	address = strings.TrimSpace(address)
	if len(address) > MaxEmailLength {
		return "", ErrEmailInvalid
	}
	parsed, err := mail.ParseAddress(address)
	if err != nil || parsed.Address != address || parsed.Name != "" {
		return "", ErrEmailInvalid
	}
	return strings.ToLower(address), nil
}
