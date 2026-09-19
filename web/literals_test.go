package web_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// styles and comments are not text a person reads, and are removed before the
// check below looks for text a person reads.
var (
	styles   = regexp.MustCompile(`(?s)<style[^>]*>.*?</style>`)
	comments = regexp.MustCompile(`(?m)^\s*//.*$`)
	// A call to another component, which renders whatever that component
	// renders and writes nothing itself.
	calls = regexp.MustCompile(`(?m)^\s*@[\p{L}_][\w.]*\(.*\)\s*\{?\s*$`)
	// A text node: something between two tags that is not an expression.
	// templ writes expressions as { ... }, so anything left with letters in it
	// was typed into the template.
	textNode = regexp.MustCompile(`>([^<>{}]*[\p{L}]{2,}[^<>{}]*)<`)
)

// TestNoTemplateHoldsItsOwnText is the check the delivery asks for: a template
// with a sentence in it is a sentence in one language, and this platform has no
// such thing (docs/requirements.md, section 6).
//
// It reads the templates rather than the generated Go, because the template is
// what a person edits and what a reviewer reads.
func TestNoTemplateHoldsItsOwnText(t *testing.T) {
	templates, err := filepath.Glob("*.templ")
	if err != nil {
		t.Fatalf("cannot look for templates: %v", err)
	}
	if len(templates) == 0 {
		t.Fatal("no template was found, so this check proves nothing")
	}

	for _, name := range templates {
		raw, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("cannot read %s: %v", name, err)
		}

		source := string(raw)
		for _, remove := range []*regexp.Regexp{styles, comments, calls} {
			source = remove.ReplaceAllString(source, "")
		}

		for _, match := range textNode.FindAllStringSubmatch(source, -1) {
			text := strings.TrimSpace(match[1])
			if text == "" {
				continue
			}
			t.Errorf("%s writes %q into the page; every user-facing string goes through a translation key", name, text)
		}
	}
}
