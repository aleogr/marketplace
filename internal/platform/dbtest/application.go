//go:build integration

package dbtest

import (
	"fmt"
	"io"
	"log/slog"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/aleogr/marketplace/internal/platform/config"
	"github.com/aleogr/marketplace/internal/platform/db"
)

// ApplicationRole stands in for the role the service connects as.
//
// The tests connect as the cluster's own superuser, which row-level security
// never applies to, and neither does it apply to a table's owner. Proving the
// policies therefore takes a third role: one that owns nothing and is nobody
// special, which is exactly what the deployment gives Cloud Run
// (infra/terraform/cloud_sql.tf).
const ApplicationRole = "app_probe"

// AsApplication creates that role and grants it what the deployment grants.
//
// The grants are not written out here: it calls the same Migrate the migration
// job calls, with the role named, so the privileges under test are the
// privileges the lab has rather than a list that drifts from them.
func AsApplication(t *testing.T, pool *db.Pool) {
	t.Helper()

	if _, err := pool.Exec(t.Context(), fmt.Sprintf(`
		DO $$
		BEGIN
		    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = '%s') THEN
		        CREATE ROLE %s NOLOGIN;
		    END IF;
		END
		$$
	`, ApplicationRole, ApplicationRole)); err != nil {
		t.Fatalf("cannot create the application role: %v", err)
	}

	var database string
	if err := pool.QueryRow(t.Context(), "SELECT current_database()").Scan(&database); err != nil {
		t.Fatalf("cannot read the database name: %v", err)
	}

	quiet := slog.New(slog.NewTextHandler(io.Discard, nil))
	settings := config.Database{URL: URL(t), Name: database, AppUser: ApplicationRole}
	if err := db.Migrate(t.Context(), pool, settings, quiet); err != nil {
		t.Fatalf("db.Migrate() = %v, want nil", err)
	}
}

// Serving runs fn as the application role, having named a marketplace — or
// named none, when marketplaceID is empty, which is what the platform's own
// rows are written under.
func Serving(t *testing.T, pool *db.Pool, marketplaceID string, fn func(pgx.Tx) error) error {
	t.Helper()

	return pool.InTx(t.Context(), func(tx pgx.Tx) error {
		// SET LOCAL, so the role and the setting end with the transaction and
		// the connection goes back to the pool as it came out.
		if _, err := tx.Exec(t.Context(), "SET LOCAL ROLE "+ApplicationRole); err != nil {
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
