// Package seo holds what a page tells a search engine and a messaging app
// about itself.
package seo

import (
	"strings"
	"unicode"
)

// Limit is where a description is cut, in runes.
//
// Runes, not bytes: Go's len() and slicing count bytes, and in Portuguese that
// cuts at around 150 characters and can split an accented letter in half,
// leaving invalid UTF-8 in the tag (docs/requirements.md, section 7.2).
//
// About 160 is what search results show. The platform cuts rather than letting
// the search engine cut, because a cut made here ends a sentence and a cut
// made there ends wherever the width ran out.
const Limit = 160

// Description returns text fit to put in a page's description.
//
// It cuts at the end of the last sentence that fits. Failing that — a first
// sentence longer than the limit — it cuts at the last word that fits and adds
// an ellipsis, because a description that ends mid-word reads as broken rather
// than as shortened.
func Description(text string) string {
	text = strings.Join(strings.Fields(text), " ")

	runes := []rune(text)
	if len(runes) <= Limit {
		return text
	}

	window := runes[:Limit]

	// The end of the last sentence inside the window. A full stop followed by
	// a space is a sentence end; one inside `R$ 1.234,56` or `marketplace1.example`
	// is not, which is why the character after it has to be a space.
	if end := lastSentence(window); end > 0 {
		return strings.TrimSpace(string(runes[:end]))
	}

	if space := lastSpace(window); space > 0 {
		return strings.TrimRight(strings.TrimSpace(string(runes[:space])), ",;:") + "…"
	}

	// One word longer than the limit. Cutting it is the only option left, and
	// cutting by rune is what keeps the result valid text.
	return string(window) + "…"
}

// lastSentence returns the index just past the last sentence end in window.
func lastSentence(window []rune) int {
	for i := len(window) - 1; i > 0; i-- {
		if !isSentenceEnd(window[i]) {
			continue
		}
		// The last rune of the window ends a sentence, or the next rune is a
		// space, which is what tells a full stop from a decimal point.
		if i == len(window)-1 || unicode.IsSpace(window[i+1]) {
			return i + 1
		}
	}
	return 0
}

func lastSpace(window []rune) int {
	for i := len(window) - 1; i > 0; i-- {
		if unicode.IsSpace(window[i]) {
			return i
		}
	}
	return 0
}

func isSentenceEnd(r rune) bool {
	switch r {
	case '.', '!', '?':
		return true
	default:
		return false
	}
}
