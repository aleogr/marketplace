// Package migrations holds the SQL that defines the database schema.
//
// The files are embedded in the binary, so the migrations that ship with a
// build are exactly the ones that build expects: there is no second artefact to
// version, to scan, or to accidentally apply from a different commit
// (docs/roadmap.md, F4).
package migrations

import "embed"

// FS holds every migration, named so that lexical order is application order.
//
//go:embed *.sql
var FS embed.FS
