// Package i18n is what the platform says, in every language it speaks.
//
// Two rules shape it (docs/requirements.md, section 6). No user-facing text
// exists outside a catalogue, so a language is added by adding a file. And
// en-US is the official language: it is what a catalogue falls back to, and
// what a visitor gets when nothing else is known.
package i18n

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"

	"golang.org/x/text/language"
	"golang.org/x/text/message"
	"golang.org/x/text/message/catalog"

	"github.com/aleogr/marketplace/web"
)

// Default is the official language, and the one a visitor gets when nothing
// about them is known (docs/requirements.md, section 6).
const Default = "en-US"

// forms is a message that reads differently for one and for many. The keys are
// CLDR's plural categories, and a catalogue gives the ones its language uses.
type forms map[string]string

// Catalogue is every language the platform speaks, loaded once.
type Catalogue struct {
	printers map[string]*message.Printer
	tags     []string
	// keys is every key of every language, for the parity check.
	keys map[string]map[string]bool
}

// Load reads the catalogues the binary carries.
func Load() (*Catalogue, error) { return load(web.Locales, "locales") }

func load(files fs.FS, dir string) (*Catalogue, error) {
	entries, err := fs.ReadDir(files, dir)
	if err != nil {
		return nil, fmt.Errorf("cannot read the catalogues: %w", err)
	}

	builder := catalog.NewBuilder(catalog.Fallback(language.AmericanEnglish))
	loaded := &Catalogue{
		printers: map[string]*message.Printer{},
		keys:     map[string]map[string]bool{},
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		tag := strings.TrimSuffix(entry.Name(), ".json")

		raw, err := fs.ReadFile(files, path.Join(dir, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("cannot read %s: %w", entry.Name(), err)
		}
		if err := loaded.add(builder, tag, raw); err != nil {
			return nil, err
		}
	}

	if len(loaded.tags) == 0 {
		return nil, fmt.Errorf("no catalogue was found in %s", dir)
	}
	sort.Strings(loaded.tags)

	for _, tag := range loaded.tags {
		parsed, err := language.Parse(tag)
		if err != nil {
			return nil, fmt.Errorf("%q is not a language tag: %w", tag, err)
		}
		loaded.printers[tag] = message.NewPrinter(parsed, message.Catalog(builder))
	}
	return loaded, nil
}

// add registers one language's messages.
func (c *Catalogue) add(builder *catalog.Builder, tag string, raw []byte) error {
	parsed, err := language.Parse(tag)
	if err != nil {
		return fmt.Errorf("%q is not a language tag: %w", tag, err)
	}

	// Into `any`, because a message is either a string or the plural forms of
	// one, and refusing anything else here is cheaper than a nil somewhere far
	// from the file that caused it.
	var messages map[string]any
	if err := json.Unmarshal(raw, &messages); err != nil {
		return fmt.Errorf("the catalogue %s is not readable JSON: %w", tag, err)
	}

	keys := map[string]bool{}
	for key, value := range messages {
		switch text := value.(type) {
		case string:
			if err := builder.SetString(parsed, key, text); err != nil {
				return fmt.Errorf("%s: cannot register %q: %w", tag, key, err)
			}
		case map[string]any:
			wordings := forms{}
			for form, message := range text {
				line, ok := message.(string)
				if !ok {
					return fmt.Errorf("%s: %q has a %s form that is not text", tag, key, form)
				}
				wordings[form] = line
			}
			if err := setPlural(builder, parsed, key, wordings); err != nil {
				return err
			}
		default:
			return fmt.Errorf("%s: %q is neither text nor plural forms", tag, key)
		}
		keys[key] = true
	}

	c.tags = append(c.tags, tag)
	c.keys[tag] = keys
	return nil
}

// Languages returns every language the platform speaks, sorted.
func (c *Catalogue) Languages() []string { return append([]string(nil), c.tags...) }

// Speaks reports whether there is a catalogue for tag.
func (c *Catalogue) Speaks(tag string) bool { _, ok := c.printers[tag]; return ok }

// Keys returns the keys of one language, for the parity check.
func (c *Catalogue) Keys(tag string) map[string]bool { return c.keys[tag] }

// Printer returns the printer of a language, falling back to the default one.
func (c *Catalogue) Printer(tag string) *message.Printer {
	if printer, ok := c.printers[tag]; ok {
		return printer
	}
	return c.printers[Default]
}
