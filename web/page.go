package web

import (
	"time"

	"golang.org/x/text/message"
)

// Page is everything a template may say or need, prepared before rendering.
//
// The templates take this and nothing else: a template that reached for a
// request, a database or a clock would be a template that behaves differently
// depending on where it is rendered, and could not be tested by rendering it.
type Page struct {
	// Language is the BCP 47 tag this page is written in, canonical case.
	Language string
	// Alternates are the same page in every other language this marketplace
	// serves, by tag, for the hreflang links search engines read.
	Alternates map[string]string
	// Default is the address a search engine should offer when it knows
	// nothing about the reader's language: the x-default link.
	Default string
	// Path is this page's own address without the language, which the
	// language switch posts back so that a visitor who switches stays where
	// they were.
	Path string

	// Marketplace is the name shown to a visitor. Empty on the platform's own
	// pages, which belong to no marketplace.
	Marketplace string
	// Nonce marks this response's own script and style, so the Content
	// Security Policy can refuse everything else.
	Nonce string
	// CSRFToken is what a form on this page sends back.
	CSRFToken string

	// printer writes this page's language.
	printer *message.Printer
	// names the languages by their own name, for the switch: a reader looking
	// for Portuguese looks for "Português", not for "pt-BR".
	names map[string]string
}

// NewPage returns a page that says things in tag.
func NewPage(tag string, printer *message.Printer, names map[string]string) Page {
	return Page{Language: tag, printer: printer, names: names, Alternates: map[string]string{}}
}

// T is what every user-facing string on a page goes through. There is no other
// way to put text on a page (docs/requirements.md, section 6).
func (p Page) T(key string, args ...any) string {
	return p.printer.Sprintf(key, args...)
}

// Name is a language's name in its own language, for the switch.
func (p Page) Name(tag string) string {
	if name, ok := p.names[tag]; ok {
		return name
	}
	return tag
}

// Languages returns every language this page exists in, this one included,
// sorted so the switch does not reorder itself between requests.
func (p Page) Languages() []string {
	tags := make([]string, 0, len(p.Alternates)+1)
	tags = append(tags, p.Language)
	for tag := range p.Alternates {
		tags = append(tags, tag)
	}
	sortStrings(tags)
	return tags
}

// Title is whose page this is: the marketplace, or the platform when the
// address belongs to no marketplace.
func (p Page) Title() string {
	if p.Marketplace != "" {
		return p.Marketplace
	}
	return p.T("page.platform.title")
}

// Year is the current year, for the footer.
func (p Page) Year() string { return time.Now().UTC().Format("2006") }

func sortStrings(values []string) {
	for i := 1; i < len(values); i++ {
		for j := i; j > 0 && values[j] < values[j-1]; j-- {
			values[j], values[j-1] = values[j-1], values[j]
		}
	}
}
