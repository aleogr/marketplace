package i18n_test

import (
	"strings"
	"testing"
	"time"

	"github.com/aleogr/marketplace/internal/platform/i18n"
)

func loaded(t *testing.T) *i18n.Catalogue {
	t.Helper()

	catalogue, err := i18n.Load()
	if err != nil {
		t.Fatalf("i18n.Load() = %v, want the catalogues", err)
	}
	return catalogue
}

// TestEveryKeyExistsInEveryLanguage is the parity check the delivery asks for,
// and it is the definition of done in one test: every user-facing text exists
// in both languages (docs/requirements.md, section 3).
func TestEveryKeyExistsInEveryLanguage(t *testing.T) {
	catalogue := loaded(t)

	languages := catalogue.Languages()
	if len(languages) < 2 {
		t.Fatalf("the platform speaks %v; the MVP speaks two languages", languages)
	}

	everything := map[string]bool{}
	for _, language := range languages {
		for key := range catalogue.Keys(language) {
			everything[key] = true
		}
	}

	for _, language := range languages {
		keys := catalogue.Keys(language)
		for key := range everything {
			if !keys[key] {
				t.Errorf("%s has no %q, which another language has", language, key)
			}
		}
	}
}

func TestBothLanguagesSayTheirOwnThing(t *testing.T) {
	catalogue := loaded(t)

	english := catalogue.Printer("en-US").Sprintf("page.home.welcome", "Marketplace 1")
	portuguese := catalogue.Printer("pt-BR").Sprintf("page.home.welcome", "Marketplace 1")

	if english != "Welcome to Marketplace 1." {
		t.Errorf("en-US = %q", english)
	}
	if portuguese != "Bem-vindo ao Marketplace 1." {
		t.Errorf("pt-BR = %q", portuguese)
	}
}

// TestAnUnknownLanguageFallsBackToTheOfficialOne: en-US is the official
// language, so a tag nobody has a catalogue for is answered in it rather than
// with an empty page (docs/requirements.md, section 6).
func TestAnUnknownLanguageFallsBackToTheOfficialOne(t *testing.T) {
	catalogue := loaded(t)

	if got := catalogue.Printer("fr-FR").Sprintf("page.home.welcome", "Marketplace 1"); got != "Welcome to Marketplace 1." {
		t.Errorf("an unknown language said %q", got)
	}
}

func TestPluralsReadCorrectlyInBothLanguages(t *testing.T) {
	catalogue := loaded(t)

	for language, want := range map[string][]string{
		"en-US": {"1 offer", "2 offers", "0 offers"},
		"pt-BR": {"1 anúncio", "2 anúncios", "0 anúncios"},
	} {
		for i, count := range []int{1, 2, 0} {
			if got := catalogue.Printer(language).Sprintf("offers.count", count); got != want[i] {
				t.Errorf("%s with %d = %q, want %q", language, count, got, want[i])
			}
		}
	}
}

// TestMoneyIsFormattedFromMinorUnits covers the rule that money is stored as a
// count of cents with its currency, never as a fraction.
func TestMoneyIsFormattedFromMinorUnits(t *testing.T) {
	catalogue := loaded(t)

	for language, want := range map[string]string{
		"pt-BR": "1.234,56",
		"en-US": "1,234.56",
	} {
		got, err := catalogue.Money(language, 123456, "BRL")
		if err != nil {
			t.Fatalf("Money() = %v", err)
		}
		if !strings.Contains(got, want) {
			t.Errorf("%s formatted 123456 minor units as %q, want it to contain %q", language, got, want)
		}
		if !strings.Contains(got, "R$") {
			t.Errorf("%s formatted BRL as %q, with no currency in it", language, got)
		}
	}
}

func TestDatesAreFormattedPerLocale(t *testing.T) {
	catalogue := loaded(t)
	when := time.Date(2026, time.September, 19, 15, 4, 0, 0, time.UTC)

	for language, want := range map[string]string{
		"en-US": "Sep 19, 2026",
		"pt-BR": "19/09/2026",
	} {
		if got := catalogue.Date(language, when, "format.date.short"); got != want {
			t.Errorf("%s = %q, want %q", language, got, want)
		}
	}

	if got := catalogue.Date("pt-BR", when, "format.date.long"); !strings.Contains(got, "setembro") {
		t.Errorf("the long Portuguese date is %q, with the month still in English", got)
	}
	if got := catalogue.Date("pt-BR", when, "format.date.long"); !strings.Contains(got, "sábado") {
		t.Errorf("the long Portuguese date is %q, with the weekday still in English", got)
	}
}
