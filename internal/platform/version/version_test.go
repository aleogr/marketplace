package version

import "testing"

// The identifier is injected at build time with -ldflags. A binary built
// without it must say so, not present an empty footer and an empty log line
// that read as "no version" rather than "unknown version".
func TestFormatNamesAnUninjectedBuild(t *testing.T) {
	tests := map[string]struct {
		injected string
		want     string
	}{
		"not injected":       {"", "unknown"},
		"injected as blanks": {"   ", "unknown"},
		"tagged build":       {"v0.3.0", "v0.3.0"},
		"after a tag":        {"v0.3.0-12-gabc1234", "v0.3.0-12-gabc1234"},
		"before any tag":     {"abc1234-20260918", "abc1234-20260918"},
		"surrounded by \\n":  {"\nv0.3.0\n", "v0.3.0"},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if got := format(test.injected); got != test.want {
				t.Errorf("format(%q) = %q, want %q", test.injected, got, test.want)
			}
		})
	}
}

func TestStringNeverReturnsAnEmptyIdentifier(t *testing.T) {
	if String() == "" {
		t.Error("String() returned an empty identifier")
	}
}
