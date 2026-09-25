// Package web holds what the interface is built from and what it says.
//
// The files are embedded rather than read from disk because the deployment is
// one binary and nothing else (docs/requirements.md, section 24): a locale that
// lived beside the binary would be a second thing to ship and a second thing to
// get wrong.
package web

import "embed"

// Locales holds one catalogue per language. A new language is a file here and
// nothing else — no code change — which is what section 6 of the requirements
// asks for.
//
//go:embed locales/*.json
var Locales embed.FS

// Mail holds one pair of files per template and language: the text part, whose
// first line is the subject, and the HTML part.
//
// They are translation files, which is the one exception to "no user-facing
// text outside a catalogue" (CLAUDE.md): a mail body is a document with
// paragraphs, and a document belongs in a file rather than in a JSON string
// with escaped newlines. A new language is a pair of files here, exactly as it
// is a catalogue there, and internal/platform/mail refuses to load a template
// that is missing in a language the platform speaks.
//
//go:embed mail/*.txt mail/*.html
var Mail embed.FS

// Assets holds the one script the site serves: the WebAuthn ceremony, which
// needs navigator.credentials and so cannot be a form alone. It is embedded,
// like everything else, and served from this origin under the page's nonce
// (internal/platform/httpx).
//
//go:embed assets/*.js
var Assets embed.FS
