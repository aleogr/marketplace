package identity

import (
	"errors"
	"strings"
	"testing"
)

// CheckLabel trims the spaces a person left around what they typed.
func TestCheckLabelTrimsSpaces(t *testing.T) {
	got, err := CheckLabel("  Celular  ")
	if err != nil || got != "Celular" {
		t.Fatalf("CheckLabel(%q) = %q, %v; want %q, nil", "  Celular  ", got, err, "Celular")
	}
}

// A label of exactly MaxLabelLength multibyte runes is kept, unchanged.
func TestCheckLabelAccepts60MultibyteRunes(t *testing.T) {
	label := strings.Repeat("é", MaxLabelLength)
	got, err := CheckLabel(label)
	if err != nil || got != label {
		t.Fatalf("CheckLabel(60 multibyte runes) = %q, %v; want the label unchanged, nil", got, err)
	}
}

// One rune over MaxLabelLength is refused, after trimming.
func TestCheckLabelRefuses61Runes(t *testing.T) {
	label := strings.Repeat("a", MaxLabelLength+1)
	if _, err := CheckLabel(label); !errors.Is(err, ErrLabelInvalid) {
		t.Fatalf("CheckLabel(61 runes) = %v, want ErrLabelInvalid", err)
	}
}

// A zero-width character (Unicode category Cf, "format") is refused: it
// changes nothing visible, which is exactly what makes it a way to hide
// something in a label two people read differently.
func TestCheckLabelRefusesAFormatCharacter(t *testing.T) {
	if _, err := CheckLabel("a\u200bb"); !errors.Is(err, ErrLabelInvalid) {
		t.Fatalf("CheckLabel with a zero-width space = %v, want ErrLabelInvalid", err)
	}
}

// A control character is refused.
func TestCheckLabelRefusesAControlCharacter(t *testing.T) {
	if _, err := CheckLabel("a\x07b"); !errors.Is(err, ErrLabelInvalid) {
		t.Fatalf("CheckLabel with a control character = %v, want ErrLabelInvalid", err)
	}
}
