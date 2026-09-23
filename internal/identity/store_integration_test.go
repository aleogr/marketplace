//go:build integration

package identity

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/aleogr/marketplace/internal/platform/config"
	"github.com/aleogr/marketplace/internal/platform/db"
	"github.com/aleogr/marketplace/internal/platform/dbtest"
	"github.com/aleogr/marketplace/internal/tenancy"
)

func TestMain(m *testing.M) { os.Exit(dbtest.Run(m)) }

// serving is a Transactor that runs as the application role, the way the
// deployed service does, so row-level security is what is tested.
type serving struct {
	t    *testing.T
	pool *db.Pool
}

func (s serving) InTxFor(_ context.Context, marketplaceID string, fn func(pgx.Tx) error) error {
	return dbtest.Serving(s.t, s.pool, marketplaceID, fn)
}

// unique returns a slug no other test has used. The tests of this package
// share one database (internal/platform/dbtest), so a fixture with a fixed
// name would find the accounts, mails and sessions an earlier test left.
func unique(t *testing.T, name string) string {
	t.Helper()
	raw := make([]byte, 6)
	if _, err := rand.Read(raw); err != nil {
		t.Fatal(err)
	}
	return name + "-" + hex.EncodeToString(raw)
}

// twoMarketplaces seeds two marketplaces of this test's own and returns their
// identifiers, in the database the package migrated.
func twoMarketplaces(t *testing.T) (serving, string, string) {
	t.Helper()
	pool, err := db.Open(t.Context(), config.Database{URL: dbtest.URL(t)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	dbtest.AsApplication(t, pool)

	one, two := unique(t, "one"), unique(t, "two")
	specs := []tenancy.Spec{
		{Slug: one, Name: "One", Market: "BR", RevenueModel: "commission", State: "active",
			DefaultLanguage: "pt-BR", Languages: []string{"pt-BR"}, Hosts: []string{one + ".test"}},
		{Slug: two, Name: "Two", Market: "BR", RevenueModel: "commission", State: "active",
			DefaultLanguage: "pt-BR", Languages: []string{"pt-BR"}, Hosts: []string{two + ".test"}},
	}
	var applied []tenancy.Applied
	if err := pool.InTx(t.Context(), func(tx pgx.Tx) error {
		var err error
		applied, err = tenancy.Seed(t.Context(), tx, specs)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return serving{t: t, pool: pool}, applied[0].ID, applied[1].ID
}

func TestAnAccountIsInvisibleFromAnotherMarketplace(t *testing.T) {
	db, one, two := twoMarketplaces(t)
	if err := db.InTxFor(t.Context(), one, func(tx pgx.Tx) error {
		_, err := insertAccount(t.Context(), tx, one, "Reader@Example.Test", "reader@example.test", "Reader")
		return err
	}); err != nil {
		t.Fatal(err)
	}

	var lookup error
	if err := db.InTxFor(t.Context(), two, func(tx pgx.Tx) error {
		_, lookup = accountByEmail(t.Context(), tx, "reader@example.test")
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(lookup, pgx.ErrNoRows) {
		t.Fatalf("looking the account up from the other marketplace = %v, want pgx.ErrNoRows", lookup)
	}

	// And the same address may open an account in the other marketplace.
	if err := db.InTxFor(t.Context(), two, func(tx pgx.Tx) error {
		_, err := insertAccount(t.Context(), tx, two, "reader@example.test", "reader@example.test", "Reader")
		return err
	}); err != nil {
		t.Fatalf("the same address could not open an account in a second marketplace: %v", err)
	}
}
