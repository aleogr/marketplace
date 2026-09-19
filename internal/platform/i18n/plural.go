package i18n

import (
	"fmt"
	"sort"
	"strings"

	"golang.org/x/text/feature/plural"
	"golang.org/x/text/language"
	"golang.org/x/text/message/catalog"
)

// categories maps CLDR's plural category names, which is what a catalogue
// writes, to what x/text selects on.
//
// English and Portuguese use `one` and `other`; the rest are here because a
// later language will need them, and a catalogue using one would otherwise
// fail to load with nothing useful to say.
var categories = map[string]plural.Form{
	"zero":  plural.Zero,
	"one":   plural.One,
	"two":   plural.Two,
	"few":   plural.Few,
	"many":  plural.Many,
	"other": plural.Other,
}

// setPlural registers a message whose wording depends on a number.
//
// The number is the first argument, by convention: a catalogue writes
// `%[1]d offer` and `%[1]d offers`, and the selection is made on that same
// argument, so no catalogue has to say which argument counts.
//
// A form may also be an exact number, written `=0`. That is not pedantry:
// CLDR puts zero in Portuguese's `one` category, which would have this
// platform say "0 anúncio", and no Brazilian writes that. The rule stays
// CLDR's and the exception is stated where a translator can see it.
func setPlural(builder *catalog.Builder, tag language.Tag, key string, wordings forms) error {
	if _, ok := wordings["other"]; !ok {
		return fmt.Errorf("%q has no `other` form, which every language needs", key)
	}

	// Exact numbers first, because selection takes the first case that
	// matches and a category would otherwise swallow them. Sorted, so that a
	// catalogue always produces the same message and never two builds that
	// differ by map order.
	var exact, named []string
	for name := range wordings {
		if strings.HasPrefix(name, "=") {
			exact = append(exact, name)
			continue
		}
		if _, known := categories[name]; !known {
			return fmt.Errorf("%q has a form %q, which is neither a plural category nor an exact number", key, name)
		}
		named = append(named, name)
	}
	sort.Strings(exact)
	sort.Strings(named)

	cases := make([]any, 0, 2*len(wordings))
	for _, name := range exact {
		cases = append(cases, name, wordings[name])
	}
	for _, name := range named {
		cases = append(cases, categories[name], wordings[name])
	}

	return builder.Set(tag, key, plural.Selectf(1, "%d", cases...))
}
