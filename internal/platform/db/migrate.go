package db

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/aleogr/marketplace/internal/platform/config"
	"github.com/aleogr/marketplace/migrations"
)

// Migrate applies every migration the binary carries that the database has not
// applied yet, and then grants the application user what the schema now holds.
//
// It is run by the `migrate` subcommand of this same binary, as a Cloud Run Job,
// before a new revision receives traffic (docs/roadmap.md, F4). Running it twice
// changes nothing the second time: goose records what it applied, and the grants
// are written to be repeatable.
func Migrate(ctx context.Context, pool *Pool, settings config.Database, log *slog.Logger) error {
	// goose speaks database/sql. Borrowing the pool rather than opening a
	// second connection keeps the Cloud SQL connector, and therefore the IAM
	// login, in one place.
	sqlDB := stdlib.OpenDBFromPool(pool.Pool)
	defer func() { _ = sqlDB.Close() }()

	goose.SetBaseFS(migrations.FS)
	goose.SetLogger(goose.NopLogger())
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("cannot prepare the migration tool: %w", err)
	}

	before, err := goose.GetDBVersionContext(ctx, sqlDB)
	if err != nil {
		return fmt.Errorf("cannot read the applied migrations: %w", err)
	}

	if err := goose.UpContext(ctx, sqlDB, "."); err != nil {
		return fmt.Errorf("cannot apply the migrations: %w", err)
	}

	after, err := goose.GetDBVersionContext(ctx, sqlDB)
	if err != nil {
		return fmt.Errorf("cannot read the applied migrations: %w", err)
	}

	log.InfoContext(ctx, "migrations applied", "from", before, "to", after)

	if settings.AppUser == "" {
		// Only the deployment that owns an application user asks for the
		// grants; a session running migrations against a local cluster is its
		// own owner and needs none.
		return nil
	}

	if err := grant(ctx, sqlDB, settings.Name, settings.AppUser); err != nil {
		return err
	}

	log.InfoContext(ctx, "privileges granted", "user", settings.AppUser)
	return nil
}

// grant gives the application user the rights the schema needs it to have, and
// arranges for tables created later to carry the same rights.
//
// Cloud SQL creates an IAM database user with no privileges at all, and
// granting privileges needs a connection that already has them — which is the
// migration user's whole reason to exist. Every statement is repeatable, so a
// re-run is a no-op rather than an error.
func grant(ctx context.Context, sqlDB *sql.DB, database, appUser string) error {
	// Identifiers cannot be bound as query parameters in PostgreSQL, so they
	// are quoted instead. pgx.Identifier escapes embedded quotes, which is what
	// makes the interpolation below safe rather than merely conventional.
	//nolint:misspell // the spelling is pgx's method name, not this project's.
	user := pgx.Identifier{appUser}.Sanitize()
	//nolint:misspell // as above.
	name := pgx.Identifier{database}.Sanitize()

	statements := []string{
		"GRANT CONNECT ON DATABASE " + name + " TO " + user,
		"GRANT USAGE ON SCHEMA public TO " + user,
		"GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO " + user,
		"GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO " + user,
		// What the next migration creates, rather than what exists now. Without
		// this, every delivery that adds a table would also have to remember to
		// grant it, and the one that forgot would fail in the lab and not in a
		// test.
		"ALTER DEFAULT PRIVILEGES IN SCHEMA public " +
			"GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO " + user,
		"ALTER DEFAULT PRIVILEGES IN SCHEMA public " +
			"GRANT USAGE, SELECT ON SEQUENCES TO " + user,
		// Functions the application calls. One of them is how host resolution
		// reads a table row-level security would otherwise hide from it
		// (migrations/00004_row_level_security.sql), and it is granted by name
		// to this role because the migration revoked it from everyone.
		"GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA public TO " + user,
		"ALTER DEFAULT PRIVILEGES IN SCHEMA public " +
			"GRANT EXECUTE ON FUNCTIONS TO " + user,
		// And one table the application may only add to. The blanket grant
		// above is applied after every migration, so a revoke written inside a
		// migration would be undone by the next deployment; here it is the
		// last word (migrations/00008_audit_log.sql). The trigger on that
		// table is the second belt, for the day a backup is restored under
		// different permissions.
		"REVOKE UPDATE, DELETE, TRUNCATE ON audit_log FROM " + user,
	}

	for _, statement := range statements {
		if _, err := sqlDB.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("cannot grant privileges to %s: %w", appUser, err)
		}
	}
	return nil
}
