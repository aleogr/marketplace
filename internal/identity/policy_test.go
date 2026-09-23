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
