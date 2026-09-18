// Package version holds the identifier of the running build.
//
// The value is injected at link time from git describe, so that the deployed
// build can always be identified from the start-up log, from the console
// footer and from the version endpoint (see docs/requirements.md, section 27).
package version

import "strings"

// version is set with -ldflags "-X github.com/aleogr/marketplace/internal/platform/version.version=...".
var version string

// String returns the identifier of this build, or "unknown" when the binary
// was linked without one.
func String() string {
	return format(version)
}

func format(injected string) string {
	trimmed := strings.TrimSpace(injected)
	if trimmed == "" {
		return "unknown"
	}
	return trimmed
}
