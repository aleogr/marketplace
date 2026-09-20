//go:build integration

package db_test

import (
	"errors"
	"io"
	"log/slog"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/aleogr/marketplace/internal/platform/config"
	"github.com/aleogr/marketplace/internal/platform/db"
	"github.com/aleogr/marketplace/internal/platform/dbtest"
)

func TestMain(m *testing.M) {
	os.Exit(dbtest.Run(m))
}

// open connects to the database dbtest made for this package, which no other
// package is migrating at the same time (internal/platform/dbtest).
func open(t *testing.T) (*db.Pool, config.Database) {
	t.Helper()

	settings := config.Database{URL: dbtest.URL(t)}
	pool, err := db.Open(t.Context(), settings)
	if err != nil {
		t.Fatalf("db.Open() = %v, want a pool", err)
	}
	t.Cleanup(pool.Close)
	return pool, settings
}

func discard() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestOpenReachesTheDatabase(t *testing.T) {
	pool, _ := open(t)

	if err := pool.Ping(t.Context()); err != nil {
		t.Fatalf("Ping() = %v, want nil", err)
	}
}

func TestOpenRefusesWhenNoDatabaseIsDeclared(t *testing.T) {
	if _, err := db.Open(t.Context(), config.Database{}); err == nil {
		t.Fatal("db.Open() with no settings returned a pool, want an error")
	}
}

func TestMigrateIsIdempotent(t *testing.T) {
	// The check the delivery asks for: applying the same migrations twice must
	// change nothing the second time. A migration tool that re-ran its files
	// would fail here on the first CREATE that is not idempotent, and would
	// otherwise only fail on the second deployment of the day.
	pool, settings := open(t)

	if err := db.Migrate(t.Context(), pool, settings, discard()); err != nil {
		t.Fatalf("first Migrate() = %v, want nil", err)
	}

	applied := versions(t, pool)
	if len(applied) == 0 {
		t.Fatal("Migrate() recorded no migrations, want at least one")
	}

	if err := db.Migrate(t.Context(), pool, settings, discard()); err != nil {
		t.Fatalf("second Migrate() = %v, want nil", err)
	}

	if again := versions(t, pool); len(again) != len(applied) {
		t.Errorf("the second Migrate() left %d migrations applied, want %d",
			len(again), len(applied))
	}
}

func TestMigrateCreatesTheSharedTriggerFunction(t *testing.T) {
	pool, settings := open(t)

	if err := db.Migrate(t.Context(), pool, settings, discard()); err != nil {
		t.Fatalf("Migrate() = %v, want nil", err)
	}

	var exists bool
	err := pool.QueryRow(t.Context(),
		`SELECT EXISTS (SELECT 1 FROM pg_proc WHERE proname = 'set_updated_at')`,
	).Scan(&exists)
	if err != nil {
		t.Fatalf("cannot read the function catalogue: %v", err)
	}
	if !exists {
		t.Error("set_updated_at() does not exist after the migrations ran")
	}
}

func TestInTxCommitsAndRollsBack(t *testing.T) {
	pool, _ := open(t)
	ctx := t.Context()

	if _, err := pool.Exec(ctx, `CREATE TABLE tx_probe (value int)`); err != nil {
		t.Fatalf("cannot create the probe table: %v", err)
	}

	if err := pool.InTx(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO tx_probe VALUES (1)`)
		return err
	}); err != nil {
		t.Fatalf("InTx() = %v, want nil", err)
	}

	wanted := errors.New("the function asked for a rollback")
	err := pool.InTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `INSERT INTO tx_probe VALUES (2)`); err != nil {
			return err
		}
		return wanted
	})
	if !errors.Is(err, wanted) {
		t.Fatalf("InTx() = %v, want the function's own error", err)
	}

	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM tx_probe`).Scan(&count); err != nil {
		t.Fatalf("cannot count the probe rows: %v", err)
	}
	if count != 1 {
		t.Errorf("tx_probe holds %d rows, want 1: the failed transaction was not rolled back", count)
	}
}

// versions lists the migrations the database records as applied.
func versions(t *testing.T, pool *db.Pool) []int64 {
	t.Helper()

	rows, err := pool.Query(t.Context(),
		`SELECT version_id FROM goose_db_version WHERE is_applied ORDER BY version_id`)
	if err != nil {
		t.Fatalf("cannot read the applied migrations: %v", err)
	}
	defer rows.Close()

	var applied []int64
	for rows.Next() {
		var version int64
		if err := rows.Scan(&version); err != nil {
			t.Fatalf("cannot read an applied migration: %v", err)
		}
		applied = append(applied, version)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("cannot read the applied migrations: %v", err)
	}
	return applied
}
