//go:build integration

package ratelimit_test

import (
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/aleogr/marketplace/internal/platform/config"
	"github.com/aleogr/marketplace/internal/platform/db"
	"github.com/aleogr/marketplace/internal/platform/dbtest"
	"github.com/aleogr/marketplace/internal/platform/ratelimit"
)

func TestMain(m *testing.M) {
	os.Exit(dbtest.Run(m))
}

// migrated returns a pool whose database carries the schema.
func migrated(t *testing.T) *db.Pool {
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
	return pool
}

// TestTheLimitHoldsAcrossInstances is why this limiter exists at all.
//
// Cloud Run runs several instances, and each holds its own memory. Two
// limiters here stand for two instances: if the counter lived in the process,
// the pair would allow the limit twice, which is how "five attempts a minute"
// becomes ten, or fifty, depending on how busy the service is
// (docs/roadmap.md, F7).
func TestTheLimitHoldsAcrossInstances(t *testing.T) {
	pool := migrated(t)

	const limit = 5
	// One fixed clock for both, so the run cannot straddle a window boundary
	// and pass for the wrong reason.
	now := time.Now()
	clock := func() time.Time { return now }

	first := ratelimit.NewDatabase(pool, limit, time.Minute)
	second := ratelimit.NewDatabase(pool, limit, time.Minute)
	ratelimit.SetDatabaseClock(first, clock)
	ratelimit.SetDatabaseClock(second, clock)

	instances := []*ratelimit.Database{first, second}
	allowed := 0
	for i := range limit * 2 {
		// Alternating, as a client hitting a service behind a load balancer
		// would be spread across the instances serving it.
		decision, err := instances[i%2].Allow(t.Context(), "sign-in:203.0.113.7")
		if err != nil {
			t.Fatalf("Allow() = %v", err)
		}
		if decision.Allowed {
			allowed++
		}
	}

	if allowed != limit {
		t.Errorf("two instances allowed %d attempts, want %d: the limit is per client, not per instance",
			allowed, limit)
	}
}

func TestARefusalSaysWhenToComeBack(t *testing.T) {
	pool := migrated(t)

	limiter := ratelimit.NewDatabase(pool, 1, time.Minute)
	now := time.Now()
	ratelimit.SetDatabaseClock(limiter, func() time.Time { return now })

	if decision, _ := limiter.Allow(t.Context(), "recovery:a"); !decision.Allowed {
		t.Fatal("the first attempt was refused")
	}

	decision, err := limiter.Allow(t.Context(), "recovery:a")
	if err != nil {
		t.Fatalf("Allow() = %v", err)
	}
	if decision.Allowed {
		t.Fatal("the second attempt was allowed")
	}
	if decision.RetryAfter <= 0 || decision.RetryAfter > 2*time.Minute {
		t.Errorf("RetryAfter = %v, want something inside the window", decision.RetryAfter)
	}
}

// TestTheNextWindowStartsClean: a limit that never forgets is a ban.
func TestTheNextWindowStartsClean(t *testing.T) {
	pool := migrated(t)

	limiter := ratelimit.NewDatabase(pool, 1, time.Minute)
	now := time.Now().Truncate(time.Minute)
	ratelimit.SetDatabaseClock(limiter, func() time.Time { return now })

	_, _ = limiter.Allow(t.Context(), "sign-in:b")
	if decision, _ := limiter.Allow(t.Context(), "sign-in:b"); decision.Allowed {
		t.Fatal("the second attempt in the window was allowed")
	}

	now = now.Add(time.Minute)
	if decision, _ := limiter.Allow(t.Context(), "sign-in:b"); !decision.Allowed {
		t.Error("the first attempt of the next window was refused")
	}
}

// TestOneClientDoesNotSpendAnothersAllowance, in the database this time.
func TestSubjectsAreCountedApart(t *testing.T) {
	pool := migrated(t)

	limiter := ratelimit.NewDatabase(pool, 1, time.Minute)
	now := time.Now()
	ratelimit.SetDatabaseClock(limiter, func() time.Time { return now })

	_, _ = limiter.Allow(t.Context(), "sign-in:203.0.113.7")
	if decision, _ := limiter.Allow(t.Context(), "sign-in:198.51.100.23"); !decision.Allowed {
		t.Error("one address spent another's allowance")
	}
}
