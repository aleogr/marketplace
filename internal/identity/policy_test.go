package identity

import (
	"errors"
	"strings"
	"testing"
)

func TestCheckPasswordCountsCharactersNotBytes(t *testing.T) {
	cases := []struct {
		password string
		want     error
	}{
		{strings.Repeat("a", MinPasswordLength-1), ErrPasswordShort},
		{strings.Repeat("a", MinPasswordLength), nil},
		{strings.Repeat("ç", MinPasswordLength), nil}, // twice as many bytes, as many characters
		{strings.Repeat("a", MaxPasswordLength), nil},
		{strings.Repeat("a", MaxPasswordLength+1), ErrPasswordLong},
		{"café com leite na padaria", nil}, // spaces allowed, no composition rule
	}
	for _, c := range cases {
		if got := CheckPassword(c.password); !errors.Is(got, c.want) {
			t.Errorf("CheckPassword(%q) = %v, want %v", c.password, got, c.want)
		}
	}
}

// The byte cap sign-in applies before normalising must never refuse a password
// that could have been set: one of MaxPasswordLength characters, typed in the
// most decomposed form that NFKC composes back.
func TestTheByteCapAdmitsEveryPasswordThatCanBeSet(t *testing.T) {
	for name, one := range map[string]string{
		"four bytes":         "\U0001F600",               // an emoji, one code point of four bytes
		"hangul jamo":        "\u1100\u1161\u11a8",       // three jamo composing to one syllable
		"stacked diacritics": "\u03b1\u0313\u0300\u0345", // four code points composing to U+1F82
	} {
		password := strings.Repeat(one, MaxPasswordLength)
		if err := CheckPassword(password); err != nil {
			t.Errorf("%s: CheckPassword = %v, want nil", name, err)
		}
		if len(password) > maxPasswordBytes {
			t.Errorf("%s: %d bytes, beyond the cap of %d", name, len(password), maxPasswordBytes)
		}
	}
}

func TestNormaliseEmail(t *testing.T) {
	good := []struct{ in, want string }{
		{"\tReader@Example.Test \n", "reader@example.test"}, // trimmed, then lower-cased
		{"a.b+tag@example.test", "a.b+tag@example.test"},
	}
	for _, c := range good {
		got, err := NormaliseEmail(c.in)
		if err != nil || got != c.want {
			t.Errorf("NormaliseEmail(%q) = %q, %v; want %q, nil", c.in, got, err, c.want)
		}
	}
	for _, bad := range []string{"", "no-at-sign", "Name <a@example.test>", "a@", "a b@example.test"} {
		if _, err := NormaliseEmail(bad); !errors.Is(err, ErrEmailInvalid) {
			t.Errorf("NormaliseEmail(%q) err = %v, want ErrEmailInvalid", bad, err)
		}
	}
}

func TestCheckNameBoundsTheLengthAndRefusesInvisibleCharacters(t *testing.T) {
	cases := []struct {
		name, want string
		err        error
	}{
		{"  Leitora  ", "Leitora", nil},
		{"   ", "", ErrNameMissing},
		{strings.Repeat("a", MaxNameLength), strings.Repeat("a", MaxNameLength), nil},
		{strings.Repeat("ç", MaxNameLength), strings.Repeat("ç", MaxNameLength), nil}, // characters, not bytes
		{strings.Repeat("a", MaxNameLength+1), "", ErrNameInvalid},
		{"Ana\nVisit https://evil.example", "", ErrNameInvalid}, // a control character
		{"Ana\u202eecilA", "", ErrNameInvalid},                  // a bidi override (format, Cf)
		{"Ana\u200bSilva", "", ErrNameInvalid},                  // a zero-width space (Cf)
		{"Ana\xffSilva", "", ErrNameInvalid},                    // not UTF-8
		{"José da Silva-Añón", "José da Silva-Añón", nil},
	}
	for _, c := range cases {
		got, err := CheckName(c.name)
		if !errors.Is(err, c.err) || (c.err == nil && got != c.want) {
			t.Errorf("CheckName(%q) = %q, %v; want %q, %v", c.name, got, err, c.want, c.err)
		}
	}
}

// RFC 5321 bounds a forward path at 256 octets, brackets included, which
// leaves 254 for the address.
func TestNormaliseEmailRefusesAnAddressLongerThanMailAllows(t *testing.T) {
	domain := "@example.test"
	longest := strings.Repeat("a", MaxEmailLength-len(domain)) + domain
	if _, err := NormaliseEmail(longest); err != nil {
		t.Errorf("NormaliseEmail(%d bytes) = %v, want nil", len(longest), err)
	}
	if _, err := NormaliseEmail("a" + longest); !errors.Is(err, ErrEmailInvalid) {
		t.Errorf("NormaliseEmail(%d bytes) err = %v, want ErrEmailInvalid", len(longest)+1, err)
	}
}

// A password beyond the byte cap is too long before it is normalised: no
// password that can be set is that long, and NFKC over a huge input is work
// spent on nothing.
func TestCheckPasswordRefusesBeyondTheByteCap(t *testing.T) {
	if err := CheckPassword(strings.Repeat("a", maxPasswordBytes+1)); !errors.Is(err, ErrPasswordLong) {
		t.Errorf("CheckPassword(beyond the byte cap) = %v, want ErrPasswordLong", err)
	}
}
