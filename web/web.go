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
