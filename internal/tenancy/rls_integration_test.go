//go:build integration

package tenancy_test

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/aleogr/marketplace/internal/platform/config"
	"github.com/aleogr/marketplace/internal/platform/db"
	"github.com/aleogr/marketplace/internal/platform/dbtest"
	"github.com/aleogr/marketplace/internal/tenancy"
)

// applicationRole stands in for the role the service connects as.
//
// The tests connect as the cluster's own superuser, which row-level security
// never applies to, and neither does it apply to a table's owner. Proving the
// policies therefore takes a third role: one that owns nothing and is nobody
// special, which is exactly what the deployment gives Cloud Run
// (infra/terraform/cloud_sql.tf).
const applicationRole = "app_probe"

// asApplication creates that role and grants it what the deployment grants.
//
// The grants are not written out here: it calls the same Migrate the migration
// job calls, with the role named, so the privileges under test are the
// privileges the lab has rather than a list that drifts from them.
func asApplication(t *testing.T, pool *db.Pool) {
	t.Helper()

	if _, err := pool.Exec(t.Context(), fmt.Sprintf(`
		DO $$
		BEGIN
		    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = '%s') THEN
		        CREATE ROLE %s NOLOGIN;
		    END IF;
		END
		$$
	`, applicationRole, applicationRole)); err != nil {
		t.Fatalf("cannot create the application role: %v", err)
	}

	var database string
	if err := pool.QueryRow(t.Context(), "SELECT current_database()").Scan(&database); err != nil {
		t.Fatalf("cannot read the database name: %v", err)
	}

	quiet := slog.New(slog.NewTextHandler(io.Discard, nil))
	settings := config.Database{URL: dbtest.URL(t), Name: database, AppUser: applicationRole}
	if err := db.Migrate(t.Context(), pool, settings, quiet); err != nil {
		t.Fatalf("db.Migrate() = %v, want nil", err)
	}
}

// serving runs fn as the application role, having named a marketplace — or
// named none, when marketplaceID is empty, which is the case the delivery is
// about.
func serving(t *testing.T, pool *db.Pool, marketplaceID string, fn func(pgx.Tx) error) error {
	t.Helper()

	return pool.InTx(t.Context(), func(tx pgx.Tx) error {
		// SET LOCAL, so the role and the setting end with the transaction and
		// the connection goes back to the pool as it came out.
		if _, err := tx.Exec(t.Context(), "SET LOCAL ROLE "+applicationRole); err != nil {
			return fmt.Errorf("cannot become the application role: %w", err)
		}
		if marketplaceID != "" {
			if _, err := tx.Exec(t.Context(),
				"SELECT set_config('app.marketplace_id', $1, true)", marketplaceID); err != nil {
				return fmt.Errorf("cannot name the marketplace: %w", err)
			}
		}
		return fn(tx)
	})
}

// ids returns the seeded marketplaces by slug.
func ids(t *testing.T, pool *db.Pool) map[string]string {
	t.Helper()

	rows, err := pool.Query(t.Context(), "SELECT slug, id::text FROM marketplace")
	if err != nil {
		t.Fatalf("cannot read the marketplaces: %v", err)
	}
	defer rows.Close()

	found := map[string]string{}
	for rows.Next() {
		var slug, id string
		if err := rows.Scan(&slug, &id); err != nil {
			t.Fatalf("cannot read a marketplace: %v", err)
		}
		found[slug] = id
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("cannot read the marketplaces: %v", err)
	}
	return found
}

func hostsVisible(t *testing.T, tx pgx.Tx) []string {
	t.Helper()

	rows, err := tx.Query(context.Background(), "SELECT host FROM marketplace_host ORDER BY host")
	if err != nil {
		t.Fatalf("cannot read the hosts: %v", err)
	}
	defer rows.Close()

	var hosts []string
	for rows.Next() {
		var host string
		if err := rows.Scan(&host); err != nil {
			t.Fatalf("cannot read a host: %v", err)
		}
		hosts = append(hosts, host)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("cannot read the hosts: %v", err)
	}
	return hosts
}

// TestAQueryThatNamesNoMarketplaceReturnsNothing is the delivery's objective.
//
// The query below has no WHERE clause at all — it is the query somebody writes
// when they forget — and the answer is zero rows rather than everyone's
// (docs/roadmap.md, F6).
func TestAQueryThatNamesNoMarketplaceReturnsNothing(t *testing.T) {
	pool := seeded(t, declared)
	asApplication(t, pool)

	if err := serving(t, pool, "", func(tx pgx.Tx) error {
		if visible := hostsVisible(t, tx); len(visible) != 0 {
			t.Errorf("a transaction naming no marketplace saw %v, want nothing", visible)
		}
		return nil
	}); err != nil {
		t.Fatalf("the transaction failed: %v", err)
	}
}

// TestEachMarketplaceSeesOnlyItsOwnRows runs the same unfiltered query under
// two marketplaces. One marketplace would prove nothing: the failure worth
// ruling out is one marketplace reading another's rows.
func TestEachMarketplaceSeesOnlyItsOwnRows(t *testing.T) {
	pool := seeded(t, declared)
	asApplication(t, pool)

	byslug := ids(t, pool)
	for slug, want := range map[string]string{
		"marketplace1": goodsHost,
		"marketplace2": vehiclesHost,
	} {
		id, ok := byslug[slug]
		if !ok {
			t.Fatalf("%s was not seeded", slug)
		}

		if err := serving(t, pool, id, func(tx pgx.Tx) error {
			visible := hostsVisible(t, tx)
			if len(visible) != 1 || visible[0] != want {
				t.Errorf("%s saw %v, want exactly [%s]", slug, visible, want)
			}
			return nil
		}); err != nil {
			t.Fatalf("the transaction for %s failed: %v", slug, err)
		}
	}
}

// TestAMarketplaceCannotWriteIntoAnother covers what USING alone would let
// through: reading is scoped by the policy's USING clause, writing by its
// WITH CHECK clause, and a policy with only the first would refuse to show a
// row it happily let you create.
func TestAMarketplaceCannotWriteIntoAnother(t *testing.T) {
	pool := seeded(t, declared)
	asApplication(t, pool)

	byslug := ids(t, pool)
	mine, theirs := byslug["marketplace1"], byslug["marketplace2"]

	err := serving(t, pool, mine, func(tx pgx.Tx) error {
		_, err := tx.Exec(t.Context(),
			"INSERT INTO marketplace_host (host, marketplace_id) VALUES ($1, $2)",
			"stolen."+platformHost, theirs)
		return err
	})
	if err == nil {
		t.Fatal("a marketplace wrote a host into another marketplace")
	}
}

// TestResolutionStillReadsEveryHost is the other half of the same policy.
//
// Resolution runs before a marketplace is known, so under the policies above
// it would see nothing and every host would answer "unknown". It reads through
// the one function that runs as the owner, and this proves that path is open
// to the application role and still returns every host.
func TestResolutionStillReadsEveryHost(t *testing.T) {
	pool := seeded(t, declared)
	asApplication(t, pool)

	if err := serving(t, pool, "", func(tx pgx.Tx) error {
		hosts, err := tenancy.NewRepository(tx).Hosts(t.Context())
		if err != nil {
			return err
		}
		if len(hosts) != 2 {
			t.Errorf("resolution read %d hosts, want 2", len(hosts))
		}
		for _, host := range []string{goodsHost, vehiclesHost} {
			if _, ok := hosts[host]; !ok {
				t.Errorf("resolution does not know %s", host)
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("resolution failed as the application role: %v", err)
	}
}

// TestEveryTenantTableIsCovered is the guard for the tables that do not exist
// yet. A later delivery that adds a table with `marketplace_id` and forgets
// the two lines that protect it fails here rather than in production
// (docs/roadmap.md, F6).
func TestEveryTenantTableIsCovered(t *testing.T) {
	pool := seeded(t, declared)

	rows, err := pool.Query(t.Context(), `
		SELECT c.relname,
		       c.relrowsecurity,
		       (SELECT count(*) FROM pg_policy p WHERE p.polrelid = c.oid)
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = 'public'
		  AND c.relkind = 'r'
		  AND (
		      EXISTS (
		          SELECT 1 FROM pg_attribute a
		          WHERE a.attrelid = c.oid AND a.attname = 'marketplace_id' AND a.attnum > 0
		      )
		      -- The marketplace itself carries no marketplace_id; it is one.
		      OR c.relname = 'marketplace'
		  )
		ORDER BY c.relname
	`)
	if err != nil {
		t.Fatalf("cannot read the schema: %v", err)
	}
	defer rows.Close()

	seen := 0
	for rows.Next() {
		var table string
		var enabled bool
		var policies int
		if err := rows.Scan(&table, &enabled, &policies); err != nil {
			t.Fatalf("cannot read a table: %v", err)
		}
		seen++

		switch {
		case !enabled:
			t.Errorf("%s belongs to a marketplace and has no row-level security", table)
		case policies == 0:
			t.Errorf("%s has row-level security and no policy, so it answers every query with nothing", table)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("cannot read the schema: %v", err)
	}

	// The tables of this delivery: marketplace, marketplace_host,
	// marketplace_language. A query that found none would pass every check
	// above while proving nothing.
	if seen < 3 {
		t.Errorf("the schema check looked at %d tables, want at least 3", seen)
	}
}
