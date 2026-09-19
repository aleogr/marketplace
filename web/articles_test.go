package web_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// glued finds a Portuguese definite article, or a preposition that carries one,
// immediately in front of the placeholder a marketplace's name is written into.
//
// The two placeholders are the ones the platform has: `%[1]s` in a catalogue
// and `{{.From}}` in a mail template.
var glued = regexp.MustCompile(`(?i)\b(o|os|as|do|da|dos|das|ao|aos|às|no|na|nos|nas|pelo|pela|num|numa)\s+(%\[1\]s|\{\{\.From\}\})`)

// TestNoPortugueseTextGluesAnArticleToAName is a rule a test can hold and a
// reviewer forgets.
//
// A marketplace's name has no predictable gender — "o Marketplace 1" reads
// correctly and "o Loja Verde" does not — and neither has the platform's own
// name. Portuguese therefore writes these sentences without the article:
// "Boas-vindas a X", "confirma que X consegue". The lapse is invisible until
// somebody reads a message addressed to a marketplace whose name happens to be
// feminine, which is why it is checked here rather than trusted to review
// (docs/requirements.md, section 6).
//
// The bare preposition `a` is not listed: "Boas-vindas a X" is correct and
// indistinguishable, by pattern, from the article. What is listed is
// unambiguous — an article, or a preposition that has swallowed one.
func TestNoPortugueseTextGluesAnArticleToAName(t *testing.T) {
	files := []string{filepath.Join("locales", "pt-BR.json")}

	entries, err := os.ReadDir("mail")
	if err != nil {
		t.Fatalf("cannot read the mail templates: %v", err)
	}
	for _, entry := range entries {
		if strings.Contains(entry.Name(), ".pt-BR.") {
			files = append(files, filepath.Join("mail", entry.Name()))
		}
	}

	for _, file := range files {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("cannot read %s: %v", file, err)
		}

		for _, found := range glued.FindAllString(string(raw), -1) {
			t.Errorf("%s writes %q: the name belongs to a marketplace, "+
				"and its gender is not this sentence's to assume", file, found)
		}
	}
}
