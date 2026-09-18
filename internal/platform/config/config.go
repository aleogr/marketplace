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

	if len(problems) > 0 {
		return Config{}, fmt.Errorf("invalid configuration: %w", errors.Join(problems...))
	}
	return cfg, nil
}
