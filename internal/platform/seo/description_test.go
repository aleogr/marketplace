package seo_test

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/aleogr/marketplace/internal/platform/seo"
)

func TestShortTextIsLeftAlone(t *testing.T) {
	const text = "Um marketplace de eletrônicos."

	if got := seo.Description(text); got != text {
		t.Errorf("Description() = %q, want it unchanged", got)
	}
}

func TestTextIsCutAtTheEndOfASentence(t *testing.T) {
	first := "Compre e venda eletrônicos com segurança no maior marketplace do Brasil."
	second := " Entrega rápida, pagamento protegido e avaliação de vendedores em cada compra."
	third := " E muito mais."

	got := seo.Description(first + second + third)

	if got != first+strings.TrimSuffix(second, " ") && got != strings.TrimSpace(first+second) {
		t.Errorf("Description() = %q, want it to end at a sentence", got)
	}
	if strings.HasSuffix(got, "…") {
		t.Error("a text with a sentence end inside the limit was cut mid-sentence")
	}
	if count := utf8.RuneCountInString(got); count > seo.Limit {
		t.Errorf("the description is %d runes, over the limit of %d", count, seo.Limit)
	}
}

// TestAccentedTextIsNeverCutInHalf is the case the requirements name: in
// Portuguese, cutting by bytes splits an accented letter and leaves invalid
// UTF-8 in the tag (section 7.2).
func TestAccentedTextIsNeverCutInHalf(t *testing.T) {
	// One long sentence of accented words, with no sentence end to fall back
	// on, so the cut lands inside the text.
	text := strings.Repeat("compra é segurança ", 30)

	got := seo.Description(text)

	if !utf8.ValidString(got) {
		t.Fatalf("the description is not valid UTF-8: %q", got)
	}
	if count := utf8.RuneCountInString(got); count > seo.Limit+1 {
		t.Errorf("the description is %d runes, over the limit of %d", count, seo.Limit)
	}
	if strings.HasSuffix(strings.TrimSuffix(got, "…"), "segur") {
		t.Error("the description ends in the middle of a word")
	}
}

// TestACutLandingOnAMultiByteCharacterStaysValid puts the limit exactly on an
// accented letter, which is where a byte-counting cut breaks.
func TestACutLandingOnAMultiByteCharacterStaysValid(t *testing.T) {
	// The 160th rune is the accented one.
	text := strings.Repeat("a", seo.Limit-1) + "ção" + strings.Repeat("b", 50)

	got := seo.Description(text)

	if !utf8.ValidString(got) {
		t.Fatalf("the description is not valid UTF-8: %q", got)
	}
	if strings.ContainsRune(got, '�') {
		t.Errorf("the description carries a replacement character: %q", got)
	}
}

func TestADecimalPointIsNotASentenceEnd(t *testing.T) {
	text := "Ofertas a partir de R$ 1.234,56 neste marketplace, com frete calculado no carrinho e pagamento protegido do começo ao fim da compra em qualquer loja."

	got := seo.Description(text + " " + strings.Repeat("mais texto ", 20))

	if strings.HasSuffix(got, "R$ 1.") {
		t.Errorf("Description() = %q; it cut at a decimal point", got)
	}
}

func TestWhitespaceIsNormalised(t *testing.T) {
	if got := seo.Description("  duas   linhas\ne  espaços  "); got != "duas linhas e espaços" {
		t.Errorf("Description() = %q", got)
	}
}
