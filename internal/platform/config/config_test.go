package config_test

import (
	"log/slog"
	"strings"
	"testing"

	"github.com/aleogr/marketplace/internal/platform/config"
)

// env builds a lookup function with the behaviour of os.LookupEnv over a map.
func env(vars map[string]string) func(string) (string, bool) {
	return func(key string) (string, bool) {
		value, ok := vars[key]
		return value, ok
	}
}

func TestLoadAppliesDefaultsWhenNothingIsSet(t *testing.T) {
	got, err := config.Load(env(nil))
	if err != nil {
		t.Fatalf("Load() returned an error: %v", err)
	}

	want := config.Config{
		Port:          8080,
		Indexable:     false,
		LogLevel:      slog.LevelInfo,
		ProvidersMode: config.ProvidersFake,
	}
	if got != want {
		t.Errorf("Load() = %+v, want %+v", got, want)
	}
}

func TestLoadReadsEveryVariable(t *testing.T) {
	got, err := config.Load(env(map[string]string{
		"PORT":           "9090",
		"INDEXABLE":      "true",
		"LOG_LEVEL":      "debug",
		"PROVIDERS_MODE": "real",
	}))
	if err != nil {
		t.Fatalf("Load() returned an error: %v", err)
	}

	want := config.Config{
		Port:          9090,
		Indexable:     true,
		LogLevel:      slog.LevelDebug,
		ProvidersMode: config.ProvidersReal,
	}
	if got != want {
		t.Errorf("Load() = %+v, want %+v", got, want)
	}
}

// A misspelled boolean must not be read as false. A deployment left silently
// non-indexable is invisible for months; a deployment that refuses to start is
// noticed immediately (see docs/requirements.md, section 7.1).
func TestLoadRefusesAMisspelledBoolean(t *testing.T) {
	_, err := config.Load(env(map[string]string{"INDEXABLE": "ture"}))
	if err == nil {
		t.Fatal("Load() accepted INDEXABLE=ture, want an error")
	}
	if !strings.Contains(err.Error(), "INDEXABLE") {
		t.Errorf("error %q does not name the variable", err)
	}
	if !strings.Contains(err.Error(), `"ture"`) {
		t.Errorf("error %q does not quote the value received", err)
	}
}

func TestLoadRefusesInvalidValues(t *testing.T) {
	tests := map[string]struct{ key, value string }{
		"port that is not a number": {"PORT", "http"},
		"port out of range":         {"PORT", "70000"},
		"unknown log level":         {"LOG_LEVEL", "verbose"},
		"unknown providers mode":    {"PROVIDERS_MODE", "sandbox"},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := config.Load(env(map[string]string{test.key: test.value}))
			if err == nil {
				t.Fatalf("Load() accepted %s=%s, want an error", test.key, test.value)
			}
			if !strings.Contains(err.Error(), test.key) {
				t.Errorf("error %q does not name %s", err, test.key)
			}
		})
	}
}

// An explicitly empty value is a mistake in the deployment configuration, not a
// request for the default. Terraform declares every variable with an explicit
// value, so an empty one means something upstream produced nothing.
func TestLoadRefusesAnEmptyValue(t *testing.T) {
	_, err := config.Load(env(map[string]string{"PORT": ""}))
	if err == nil {
		t.Fatal("Load() accepted an empty PORT, want an error")
	}
	if !strings.Contains(err.Error(), "PORT") {
		t.Errorf("error %q does not name the variable", err)
	}
}

// Reporting one problem per restart turns three typos into three deployments.
func TestLoadReportsEveryInvalidVariableAtOnce(t *testing.T) {
	_, err := config.Load(env(map[string]string{
		"PORT":           "http",
		"INDEXABLE":      "ture",
		"PROVIDERS_MODE": "sandbox",
	}))
	if err == nil {
		t.Fatal("Load() accepted three invalid values, want an error")
	}
	for _, key := range []string{"PORT", "INDEXABLE", "PROVIDERS_MODE"} {
		if !strings.Contains(err.Error(), key) {
			t.Errorf("error %q does not name %s", err, key)
		}
	}
}
