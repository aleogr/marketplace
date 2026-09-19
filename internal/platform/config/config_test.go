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
		ProxyHops:     2,
	}
	if got != want {
		t.Errorf("Load() = %+v, want %+v", got, want)
	}
}

func TestLoadReadsEveryVariable(t *testing.T) {
	got, err := config.Load(env(map[string]string{
		"PORT":               "9090",
		"INDEXABLE":          "true",
		"LOG_LEVEL":          "debug",
		"PROVIDERS_MODE":     "real",
		"TRUSTED_PROXY_HOPS": "1",
		"MAIL_FROM":          "no-reply@marketplace.example",
		"MAIL_API_KEY":       "a-key",
		"MAIL_DIRECTORY":     "/tmp/mailbox",
		"MAIL_WEBHOOK_TOKEN": "a-token",
		"AUDIT_KEY":          "projects/p/locations/l/keyRings/r/cryptoKeys/k",
	}))
	if err != nil {
		t.Fatalf("Load() returned an error: %v", err)
	}

	want := config.Config{
		Port:          9090,
		Indexable:     true,
		LogLevel:      slog.LevelDebug,
		ProvidersMode: config.ProvidersReal,
		ProxyHops:     1,
		Mail: config.Mail{
			From:         "no-reply@marketplace.example",
			Key:          "a-key",
			Directory:    "/tmp/mailbox",
			WebhookToken: "a-token",
		},
		Audit: config.Audit{Key: "projects/p/locations/l/keyRings/r/cryptoKeys/k"},
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

func TestLoadRefusesContradictoryDatabaseSettings(t *testing.T) {
	t.Parallel()

	for name, environment := range map[string]map[string]string{
		"both routes at once": {
			"DATABASE_URL":      "postgres://localhost/marketplace",
			"DATABASE_INSTANCE": "project:region:instance",
			"DATABASE_NAME":     "marketplace",
			"DATABASE_USER":     "service",
		},
		"an instance without a database name": {
			"DATABASE_INSTANCE": "project:region:instance",
			"DATABASE_USER":     "service",
		},
		"an instance without a user": {
			"DATABASE_INSTANCE": "project:region:instance",
			"DATABASE_NAME":     "marketplace",
		},
		"settings with no route to use them": {
			"DATABASE_NAME": "marketplace",
			"DATABASE_USER": "service",
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if _, err := config.Load(env(environment)); err == nil {
				t.Errorf("Load(%v) started, want a refusal", environment)
			}
		})
	}
}

func TestLoadAcceptsEitherRouteToTheDatabase(t *testing.T) {
	t.Parallel()

	direct, err := config.Load(env(map[string]string{
		"DATABASE_URL": "postgres://localhost/marketplace",
	}))
	if err != nil {
		t.Fatalf("Load() with DATABASE_URL = %v, want nil", err)
	}
	if !direct.Database.Configured() {
		t.Error("a declared DATABASE_URL did not configure a database")
	}

	connector, err := config.Load(env(map[string]string{
		"DATABASE_INSTANCE": "project:region:instance",
		"DATABASE_NAME":     "marketplace",
		"DATABASE_USER":     "service",
	}))
	if err != nil {
		t.Fatalf("Load() with DATABASE_INSTANCE = %v, want nil", err)
	}
	if !connector.Database.Configured() {
		t.Error("a declared DATABASE_INSTANCE did not configure a database")
	}
}

func TestLoadWithoutADatabaseIsAValidConfiguration(t *testing.T) {
	t.Parallel()

	cfg, err := config.Load(env(map[string]string{}))
	if err != nil {
		t.Fatalf("Load() = %v, want nil", err)
	}
	if cfg.Database.Configured() {
		t.Error("an empty environment configured a database")
	}
}

// A deployment that says it uses real providers and was given no e-mail
// credential sends nothing, and discovers it the first time somebody is
// supposed to receive something. Refusing to start is seconds; a sign-up
// confirmation that never arrives is a person who cannot use the platform
// (docs/requirements.md, section 7.1).
func TestLoadRefusesRealProvidersWithNoMailCredential(t *testing.T) {
	for name, vars := range map[string]map[string]string{
		"no key at all": {"PROVIDERS_MODE": "real"},
		"a key and no address": {
			"PROVIDERS_MODE": "real",
			"MAIL_API_KEY":   "a-key",
		},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := config.Load(env(vars)); err == nil {
				t.Fatal("Load() accepted real providers with no e-mail settings, want an error")
			}
		})
	}
}

// And the fake adapter needs none of it: an environment whose provider account
// is not ready yet is the mode this platform runs in until F11's owner steps
// are done (docs/roadmap.md, F11).
func TestLoadAcceptsFakeProvidersWithNoMailCredential(t *testing.T) {
	if _, err := config.Load(env(map[string]string{"PROVIDERS_MODE": "fake"})); err != nil {
		t.Fatalf("Load() = %v, want no error", err)
	}
}

// An audit log protected by a key this process holds is a log anyone who reads
// the environment can open, and a deletion request against it means nothing
// (docs/requirements.md, section 21).
func TestLoadRefusesRealProvidersWithoutAKeyManager(t *testing.T) {
	providers := map[string]string{
		"PROVIDERS_MODE": "real",
		"MAIL_API_KEY":   "a-key",
		"MAIL_FROM":      "no-reply@marketplace.example",
	}

	if _, err := config.Load(env(providers)); err == nil {
		t.Error("Load() accepted real providers with no key manager")
	}

	both := map[string]string{"AUDIT_KEY": "a-key-name", "AUDIT_LOCAL_KEY": "a-local-key"}
	for name, value := range providers {
		both[name] = value
	}
	if _, err := config.Load(env(both)); err == nil {
		t.Error("Load() accepted a key manager and a local key at once")
	}
}
