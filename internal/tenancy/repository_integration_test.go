//go:build integration

package tenancy_test

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/aleogr/marketplace/internal/platform/config"
	"github.com/aleogr/marketplace/internal/platform/db"
	"github.com/aleogr/marketplace/internal/platform/dbtest"
	"github.com/aleogr/marketplace/internal/tenancy"
)

func TestMain(m *testing.M) {
	os.Exit(dbtest.Run(m))
}

const (
	goodsHost    = "marketplace1." + platformHost
	vehiclesHost = "marketplace2." + platformHost
)

// The two marketplaces differ in everything the resolution carries, so that an
// answer naming the wrong one differs in more than its name.
var declared = []tenancy.Spec{
	{
		Slug: "marketplace1", Name: "Marketplace 1", Market: "BR",
		RevenueModel: "commission", DefaultLanguage: "pt-BR",
		Languages: []string{"en-US", "pt-BR"}, Hosts: []string{goodsHost},
		DetectContactData: true, RevealContact: false,
	},
	{
		Slug: "marketplace2", Name: "Marketplace 2", Market: "BR",
		RevenueModel: "paid_listings", DefaultLanguage: "pt-BR",
		Languages: []string{"en-US", "pt-BR"}, Hosts: []string{vehiclesHost},
		DetectContactData: false, RevealContact: true,
	},
}

// seeded returns a pool whose database has the schema and the marketplaces
// above, exactly as the migration job leaves it.
func seeded(t *testing.T, specs []tenancy.Spec) *db.Pool {
	t.Helper()

	settings := config.Database{URL: dbtest.URL(t)}
	pool, err := db.Open(t.Context(), settings)
	if err != nil {
		t.Fatalf("db.Open() = %v, want a pool", err)
	}
	t.Cleanup(pool.Close)

	quiet := slog.New(slog.NewTextHandler(io.Discard, nil))
	if err := db.Migrate(t.Context(), pool, settings, quiet); err != nil {
		t.Fatalf("db.Migrate() = %v, want nil", err)
	}

	if err := pool.InTx(t.Context(), func(tx pgx.Tx) error {
		return tenancy.Seed(t.Context(), tx, specs)
	}); err != nil {
		t.Fatalf("tenancy.Seed() = %v, want nil", err)
	}
	return pool
}

// TestTwoMarketplacesReachTheirOwnContext is the check the delivery asks for.
//
// One marketplace proves nothing about isolation: the failure worth ruling out
// is a request arriving on one host and being served as another, and it takes
// two marketplaces to see it.
func TestTwoMarketplacesReachTheirOwnContext(t *testing.T) {
	pool := seeded(t, declared)
	resolver := tenancy.NewResolver(tenancy.NewRepository(pool), platformHost)

	for host, want := range map[string]tenancy.Spec{
		goodsHost:    declared[0],
		vehiclesHost: declared[1],
	} {
		var served *tenancy.Marketplace
		handler := tenancy.Resolve(resolver, &pages{})(
			http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				resolution, _ := tenancy.FromContext(r.Context())
				served = resolution.Marketplace
			}))

		handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequestWithContext(
			t.Context(), http.MethodGet, "http://"+host+"/", nil))

		if served == nil {
			t.Fatalf("%s was not served a marketplace", host)
		}
		if served.Slug != want.Slug {
			t.Errorf("%s served %q, want %q", host, served.Slug, want.Slug)
		}
		if served.RevenueModel != want.RevenueModel {
			t.Errorf("%s served revenue model %q, want %q",
				host, served.RevenueModel, want.RevenueModel)
		}
		if served.RevealContact != want.RevealContact {
			t.Errorf("%s served reveal-contact %v, want %v",
				host, served.RevealContact, want.RevealContact)
		}
	}
}

// TestSeedIsRepeatable matters because every deployment runs it. A seed that
// only worked on an empty database would fail on the second merge of the day.
func TestSeedIsRepeatable(t *testing.T) {
	pool := seeded(t, declared)

	if err := pool.InTx(t.Context(), func(tx pgx.Tx) error {
		return tenancy.Seed(t.Context(), tx, declared)
	}); err != nil {
		t.Fatalf("the second Seed() = %v, want nil", err)
	}

	hosts, err := tenancy.NewRepository(pool).Hosts(t.Context())
	if err != nil {
		t.Fatalf("Hosts() = %v, want the host map", err)
	}
	if len(hosts) != 2 {
		t.Errorf("the host map holds %d hosts, want 2: seeding twice duplicated them", len(hosts))
	}
}

// TestSeedRemovesAHostTheEnvironmentNoLongerDeclares is what makes the
// environment authoritative. A host left behind would keep resolving to a
// marketplace nobody is pointing DNS at.
func TestSeedRemovesAHostTheEnvironmentNoLongerDeclares(t *testing.T) {
	pool := seeded(t, declared)

	withdrawn := []tenancy.Spec{declared[0], declared[1]}
	withdrawn[1].Hosts = []string{"marketplace2-renamed." + platformHost}

	if err := pool.InTx(t.Context(), func(tx pgx.Tx) error {
		return tenancy.Seed(t.Context(), tx, withdrawn)
	}); err != nil {
		t.Fatalf("Seed() = %v, want nil", err)
	}

	hosts, err := tenancy.NewRepository(pool).Hosts(t.Context())
	if err != nil {
		t.Fatalf("Hosts() = %v, want the host map", err)
	}
	if _, still := hosts[vehiclesHost]; still {
		t.Errorf("%s still resolves after the environment stopped declaring it", vehiclesHost)
	}
	if _, ok := hosts["marketplace2-renamed."+platformHost]; !ok {
		t.Error("the newly declared host does not resolve")
	}
}

func TestSeedCarriesBothLanguages(t *testing.T) {
	pool := seeded(t, declared)

	hosts, err := tenancy.NewRepository(pool).Hosts(t.Context())
	if err != nil {
		t.Fatalf("Hosts() = %v, want the host map", err)
	}

	marketplace, ok := hosts[goodsHost]
	if !ok {
		t.Fatal("the seeded host is not in the map")
	}

	// Both languages from day one is the definition of done, not a later
	// nicety (docs/requirements.md, section 6).
	want := map[string]bool{"en-US": true, "pt-BR": true}
	if len(marketplace.Languages) != len(want) {
		t.Fatalf("languages = %v, want %v", marketplace.Languages, want)
	}
	for _, language := range marketplace.Languages {
		if !want[language] {
			t.Errorf("languages carry %q, which is not one of %v", language, want)
		}
	}
}

func TestAnUnknownHostResolvesToNothing(t *testing.T) {
	pool := seeded(t, declared)
	resolver := tenancy.NewResolver(tenancy.NewRepository(pool), platformHost)

	resolution, err := resolver.Resolve(t.Context(), "someone-elses-domain.example")
	if err != nil {
		t.Fatalf("Resolve() = %v, want nil", err)
	}
	if resolution.Kind != tenancy.Unknown {
		t.Errorf("an unconfigured host resolved to %v, want Unknown", resolution.Kind)
	}
}
