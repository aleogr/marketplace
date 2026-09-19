// Command marketplace is the platform's single binary.
//
// With no arguments it serves. With `migrate` it applies the database
// migrations it carries and exits. There is one binary and therefore one image:
// the schema a deployment applies and the code that expects it are always from
// the same commit (docs/roadmap.md, F4).
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/aleogr/marketplace/internal/platform/config"
	"github.com/aleogr/marketplace/internal/platform/db"
	"github.com/aleogr/marketplace/internal/platform/geoip"
	"github.com/aleogr/marketplace/internal/platform/httpx"
	"github.com/aleogr/marketplace/internal/platform/i18n"
	"github.com/aleogr/marketplace/internal/platform/logging"
	"github.com/aleogr/marketplace/internal/platform/ratelimit"
	"github.com/aleogr/marketplace/internal/platform/version"
	"github.com/aleogr/marketplace/internal/tenancy"
)

// shutdownGrace is how long in-flight requests have to finish after SIGTERM.
// Cloud Run allows up to ten seconds before it kills the instance.
const shutdownGrace = 8 * time.Second

// The general limit, per client address and per instance. It is deliberately
// generous: it is there to blunt a flood, not to pace a visitor who is loading
// a page with its images and a few HTMX fragments. The sensitive endpoints are
// bounded far more tightly, and in the database, by the delivery that adds them
// (docs/roadmap.md, F7 and F10). Both become console parameters in F17.
const (
	generalBurst     = 120
	generalPerMinute = 240
)

func main() {
	if err := run(context.Background(), os.Args[1:], os.LookupEnv, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "marketplace: %v\n", err)
		os.Exit(1)
	}
}

// run reads the configuration and does what the arguments ask for.
func run(ctx context.Context, args []string, lookup config.Lookup, stdout io.Writer) error {
	cfg, err := config.Load(lookup)
	if err != nil {
		return err
	}

	log := logging.New(stdout, cfg.LogLevel)

	if len(args) == 0 {
		return serve(ctx, cfg, log)
	}

	switch args[0] {
	case "migrate":
		return migrate(ctx, cfg, log)
	default:
		return fmt.Errorf(
			"unknown command %q: the binary serves when given no arguments, and applies migrations with `migrate`",
			args[0])
	}
}

// serve runs the HTTP server until the process is asked to stop.
func serve(ctx context.Context, cfg config.Config, log *slog.Logger) error {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	// The pool is opened before the listener so that a database that was
	// declared and cannot be reached refuses the start rather than answering
	// health checks with a 503 nobody is watching for.
	var database *db.Pool
	if cfg.Database.Configured() {
		pool, err := db.Open(ctx, cfg.Database)
		if err != nil {
			return err
		}
		defer pool.Close()

		if err := pool.Ping(ctx); err != nil {
			return fmt.Errorf("the database was declared but does not answer: %w", err)
		}
		database = pool
	}

	var listenConfig net.ListenConfig
	listener, err := listenConfig.Listen(ctx, "tcp", fmt.Sprintf(":%d", cfg.Port))
	if err != nil {
		return fmt.Errorf("cannot listen on port %d: %w", cfg.Port, err)
	}

	log.InfoContext(ctx, "server started",
		"version", version.String(),
		"port", cfg.Port,
		"indexable", cfg.Indexable,
		"providers", string(cfg.ProvidersMode),
		// Which of the two modes is in effect, on every start, for the same
		// reason the indexing mode is logged: a process quietly running without
		// a database is a thing somebody must be able to see.
		"database", databaseMode(cfg.Database),
		"platform_host", cfg.PlatformHost,
	)

	// The catalogues are read once, at start-up. A catalogue that cannot be
	// read is a process that cannot say anything to anybody, so it refuses to
	// start rather than serving pages of empty strings.
	catalogue, err := i18n.Load()
	if err != nil {
		return err
	}

	// A nil *db.Pool in an interface is not a nil interface, so the handler is
	// given one only when there is one to give.
	var checker httpx.Database
	if database != nil {
		checker = database
	}

	pages := httpx.NewPages(catalogue)
	handler := httpx.NewSite(checker, catalogue).Handler()

	// Language comes next, and it redirects: the language is part of the URL,
	// so a page is never served at an address that does not say which language
	// it is in (docs/design.md, decision 10). Country detection is a port with
	// no adapter yet, which is a working deployment — everyone gets the
	// official language until they choose otherwise (docs/roadmap.md, F18).
	handler = i18n.NewResolver(catalogue, geoip.Nowhere{}).
		Resolve(httpx.Speaks(catalogue), httpx.HealthPath, httpx.LanguagePath)(handler)

	// Host resolution comes before that, because language, session and every
	// query after it are scoped by the answer (docs/design.md, section 2.3).
	// It needs the database to know the hosts, so a process running without
	// one serves every host as before.
	if database != nil {
		resolver := tenancy.NewResolver(tenancy.NewRepository(database), cfg.PlatformHost)
		handler = tenancy.Resolve(resolver, pages, httpx.HealthPath)(handler)
	}

	// And the defences come before that. There is no load balancer and no web
	// application firewall in front of this process, so the binary is what
	// applies them (docs/requirements.md, section 24).
	handler = httpx.Pipeline{
		Indexable: cfg.Indexable,
		ProxyHops: cfg.ProxyHops,
		General:   ratelimit.NewMemory(generalBurst, generalPerMinute),
		Pages:     pages,
		Log:       log,
	}.Wrap(handler)

	if err := httpx.Serve(ctx, listener, handler, shutdownGrace); err != nil {
		return err
	}

	log.InfoContext(context.WithoutCancel(ctx), "server stopped")
	return nil
}

// migrate applies the migrations the binary carries and returns.
func migrate(ctx context.Context, cfg config.Config, log *slog.Logger) error {
	if !cfg.Database.Configured() {
		return errors.New(
			"`migrate` needs a database: set DATABASE_URL, or DATABASE_INSTANCE with DATABASE_NAME and DATABASE_USER")
	}

	pool, err := db.Open(ctx, cfg.Database)
	if err != nil {
		return err
	}
	defer pool.Close()

	log.InfoContext(ctx, "applying migrations",
		"version", version.String(),
		"database", databaseMode(cfg.Database),
	)

	if err := db.Migrate(ctx, pool, cfg.Database, log); err != nil {
		return err
	}

	// The marketplaces this environment declares, applied after the schema
	// that holds them. They come from the environment rather than from a
	// migration because their hosts are the environment's: a host written into
	// a versioned migration would be created in every environment that runs it
	// (internal/tenancy/seed.go).
	specs, err := tenancy.ParseSpecs(cfg.SeedMarketplaces)
	if err != nil {
		return err
	}
	if len(specs) == 0 {
		log.InfoContext(ctx, "no marketplaces declared for this environment")
		return nil
	}

	if err := pool.InTx(ctx, func(tx pgx.Tx) error {
		return tenancy.Seed(ctx, tx, specs)
	}); err != nil {
		return err
	}

	slugs := make([]string, 0, len(specs))
	for _, spec := range specs {
		slugs = append(slugs, spec.Slug)
	}
	log.InfoContext(ctx, "marketplaces seeded", "slugs", slugs)
	return nil
}

// databaseMode names the route to the database without ever naming a
// credential: the connection string may carry a password, so it is the route
// that is logged and not the setting.
func databaseMode(settings config.Database) string {
	switch {
	case settings.Instance != "":
		return "cloud sql: " + settings.Instance
	case settings.URL != "":
		return "direct"
	default:
		return "none"
	}
}
