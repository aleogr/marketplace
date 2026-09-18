// Package config loads the process configuration from the environment.
//
// Loading is strict: a variable that is set but cannot be read is an error, and
// never a silent fallback to the default. The errors are not symmetric. A
// deployment that refuses to start is noticed in seconds; a deployment that
// started with a misread value can stay wrong for months (see
// docs/requirements.md, section 7.1).
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"strconv"
)

// ProvidersMode selects between the real external providers and the fakes used
// by tests and by an environment whose provider accounts are not ready yet.
type ProvidersMode string

const (
	ProvidersFake ProvidersMode = "fake"
	ProvidersReal ProvidersMode = "real"
)

// Database is how the process reaches PostgreSQL.
//
// Exactly one of the two routes may be declared. URL is a direct connection,
// which is what a cluster in a session and a service container in CI offer.
// Instance is a Cloud SQL connection name, reached through the connector, which
// authenticates with IAM and needs no network of its own (docs/design.md,
// section 3).
//
// Neither declared means the process runs without a database. That is a real
// mode, not an oversight — the end-to-end suite starts the binary that way —
// and the start-up log says which mode is in effect on every start, for the
// same reason the indexing mode is logged (docs/requirements.md, section 7.1).
type Database struct {
	URL      string
	Instance string
	Name     string
	User     string
	Password string
	// AppUser is the database user the migrations grant privileges to. Only
	// the migration job sets it: a Cloud SQL IAM user is created with no
	// rights at all, and granting rights needs a connection that has them.
	AppUser string
}

// Configured reports whether a database was declared at all.
func (d Database) Configured() bool { return d.URL != "" || d.Instance != "" }

// Config is the process configuration. Every field has a default that is safe
// for an environment where nothing was declared.
type Config struct {
	// Port is the TCP port the HTTP server listens on. Cloud Run sets PORT.
	Port int
	// Indexable allows search engines to list this deployment's pages.
	Indexable bool
	// LogLevel is the minimum severity written to the log.
	LogLevel slog.Level
	// ProvidersMode selects the real provider adapters or the fakes.
	ProvidersMode ProvidersMode
	// Database is how the process reaches PostgreSQL, if it does at all.
	Database Database
}

// Lookup reports the value of an environment variable and whether it was set.
// os.LookupEnv satisfies it.
type Lookup func(key string) (string, bool)

// Load reads the configuration from lookup.
//
// Every invalid variable is reported in a single error: reporting one problem
// per restart turns three typos into three deployments.
func Load(lookup Lookup) (Config, error) {
	cfg := Config{
		Port:          8080,
		Indexable:     false,
		LogLevel:      slog.LevelInfo,
		ProvidersMode: ProvidersFake,
	}

	var problems []error
	read := func(key string, parse func(string) error) {
		value, ok := lookup(key)
		if !ok {
			return
		}
		if value == "" {
			problems = append(problems, fmt.Errorf("%s is set but empty", key))
			return
		}
		if err := parse(value); err != nil {
			problems = append(problems, fmt.Errorf("%s: %w", key, err))
		}
	}

	read("PORT", func(value string) error {
		port, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("%q is not a number", value)
		}
		if port < 1 || port > 65535 {
			return fmt.Errorf("%q is outside the range 1-65535", value)
		}
		cfg.Port = port
		return nil
	})

	read("INDEXABLE", func(value string) error {
		indexable, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("%q is not a boolean; use true or false", value)
		}
		cfg.Indexable = indexable
		return nil
	})

	read("LOG_LEVEL", func(value string) error {
		var level slog.Level
		if err := level.UnmarshalText([]byte(value)); err != nil {
			return fmt.Errorf("%q is not a level; use debug, info, warn or error", value)
		}
		cfg.LogLevel = level
		return nil
	})

	read("PROVIDERS_MODE", func(value string) error {
		switch ProvidersMode(value) {
		case ProvidersFake, ProvidersReal:
			cfg.ProvidersMode = ProvidersMode(value)
			return nil
		default:
			return fmt.Errorf("%q is not a mode; use fake or real", value)
		}
	})

	read("DATABASE_URL", func(value string) error {
		cfg.Database.URL = value
		return nil
	})

	read("DATABASE_INSTANCE", func(value string) error {
		cfg.Database.Instance = value
		return nil
	})

	read("DATABASE_NAME", func(value string) error {
		cfg.Database.Name = value
		return nil
	})

	read("DATABASE_USER", func(value string) error {
		cfg.Database.User = value
		return nil
	})

	read("DATABASE_PASSWORD", func(value string) error {
		cfg.Database.Password = value
		return nil
	})

	read("DATABASE_APP_USER", func(value string) error {
		cfg.Database.AppUser = value
		return nil
	})

	problems = append(problems, cfg.Database.problems()...)

	if len(problems) > 0 {
		return Config{}, fmt.Errorf("invalid configuration: %w", errors.Join(problems...))
	}
	return cfg, nil
}

// problems reports every way the declared database settings contradict each
// other. A half-declared database is refused at start-up rather than discovered
// on the first query, which in a deployment means minutes instead of whenever
// someone next looks (docs/requirements.md, section 7.1).
func (d Database) problems() []error {
	var problems []error

	if d.URL != "" && d.Instance != "" {
		problems = append(problems, errors.New(
			"DATABASE_URL and DATABASE_INSTANCE are both set; a database is reached one way or the other"))
	}

	if d.Instance != "" {
		if d.Name == "" {
			problems = append(problems, errors.New("DATABASE_INSTANCE is set but DATABASE_NAME is not"))
		}
		if d.User == "" {
			problems = append(problems, errors.New("DATABASE_INSTANCE is set but DATABASE_USER is not"))
		}
	}

	if !d.Configured() {
		for key, value := range map[string]string{
			"DATABASE_NAME":     d.Name,
			"DATABASE_USER":     d.User,
			"DATABASE_PASSWORD": d.Password,
			"DATABASE_APP_USER": d.AppUser,
		} {
			if value != "" {
				problems = append(problems, fmt.Errorf(
					"%s is set but neither DATABASE_URL nor DATABASE_INSTANCE is", key))
			}
		}
	}

	return problems
}
