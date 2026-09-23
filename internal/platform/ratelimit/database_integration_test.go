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

	first := ratelimit.NewDatabase(pool, "sign-in", limit, time.Minute)
	second := ratelimit.NewDatabase(pool, "sign-in", limit, time.Minute)
	ratelimit.SetDatabaseClock(first, clock)
	ratelimit.SetDatabaseClock(second, clock)

	instances := []*ratelimit.Database{first, second}
	allowed := 0
	for i := range limit * 2 {
		// Alternating, as a client hitting a service behind a load balancer
		// would be spread across the instances serving it.
		decision, err := instances[i%2].Allow(t.Context(), "203.0.113.7")
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

	limiter := ratelimit.NewDatabase(pool, "recovery", 1, time.Minute)
	now := time.Now()
	ratelimit.SetDatabaseClock(limiter, func() time.Time { return now })

	if decision, _ := limiter.Allow(t.Context(), "a"); !decision.Allowed {
		t.Fatal("the first attempt was refused")
	}

	decision, err := limiter.Allow(t.Context(), "a")
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

	limiter := ratelimit.NewDatabase(pool, "sign-in", 1, time.Minute)
	now := time.Now().Truncate(time.Minute)
	ratelimit.SetDatabaseClock(limiter, func() time.Time { return now })

	_, _ = limiter.Allow(t.Context(), "b")
	if decision, _ := limiter.Allow(t.Context(), "b"); decision.Allowed {
		t.Fatal("the second attempt in the window was allowed")
	}

	now = now.Add(time.Minute)
	if decision, _ := limiter.Allow(t.Context(), "b"); !decision.Allowed {
		t.Error("the first attempt of the next window was refused")
	}
}

// TestOneClientDoesNotSpendAnothersAllowance, in the database this time.
func TestSubjectsAreCountedApart(t *testing.T) {
	pool := migrated(t)

	limiter := ratelimit.NewDatabase(pool, "sign-in", 1, time.Minute)
	now := time.Now()
	ratelimit.SetDatabaseClock(limiter, func() time.Time { return now })

	_, _ = limiter.Allow(t.Context(), "203.0.113.7")
	if decision, _ := limiter.Allow(t.Context(), "198.51.100.23"); !decision.Allowed {
		t.Error("one address spent another's allowance")
	}
}

// TestALimiterForgetsOnlyItsOwnWindows is the regression test of F13: the
// sweep used to forget every row older than its own window, so a limiter
// with a ten-minute window, opening one, erased the live counters of an
// hourly limiter and handed it a fresh allowance.
func TestALimiterForgetsOnlyItsOwnWindows(t *testing.T) {
	pool := migrated(t)

	hour := time.Now().UTC().Truncate(time.Hour)
	now := hour.Add(5 * time.Minute)
	clock := func() time.Time { return now }

	hourly := ratelimit.NewDatabase(pool, "hourly", 1, time.Hour)
	short := ratelimit.NewDatabase(pool, "short", 1, 10*time.Minute)
	ratelimit.SetDatabaseClock(hourly, clock)
	ratelimit.SetDatabaseClock(short, clock)

	if decision, err := hourly.Allow(t.Context(), "203.0.113.7"); err != nil || !decision.Allowed {
		t.Fatalf("the first hourly attempt = %+v, %v; want allowed", decision, err)
	}

	// Twenty-five minutes on, the short limiter opens a window. Its sweep
	// forgets rows older than ten minutes before that window, and the hourly
	// row, from 5 past, is one of them by age but not by ownership.
	now = hour.Add(25 * time.Minute)
	if decision, err := short.Allow(t.Context(), "203.0.113.7"); err != nil || !decision.Allowed {
		t.Fatalf("the short limiter's attempt = %+v, %v; want allowed", decision, err)
	}

	now = hour.Add(30 * time.Minute)
	decision, err := hourly.Allow(t.Context(), "203.0.113.7")
	if err != nil {
		t.Fatalf("Allow() = %v", err)
	}
	if decision.Allowed {
		t.Fatal("the hourly limit was reset by another limiter's sweep")
	}
}

// TestALimiterStillForgetsItsOwnPastWindows: the sweep still does its job.
func TestALimiterStillForgetsItsOwnPastWindows(t *testing.T) {
	pool := migrated(t)

	now := time.Now().UTC().Truncate(time.Minute)
	limiter := ratelimit.NewDatabase(pool, "sweep", 1, time.Minute)
	ratelimit.SetDatabaseClock(limiter, func() time.Time { return now })

	_, _ = limiter.Allow(t.Context(), "a")
	now = now.Add(3 * time.Minute)
	_, _ = limiter.Allow(t.Context(), "a")

	var left int
	if err := pool.QueryRow(t.Context(),
		"SELECT count(*) FROM rate_limit WHERE subject = 'sweep:a'").Scan(&left); err != nil {
		t.Fatal(err)
	}
	if left != 1 {
		t.Fatalf("%d rows of this limiter are left, want only the open window's", left)
	}
}
