package ratelimit_test

import (
	"testing"
	"time"

	"github.com/aleogr/marketplace/internal/platform/ratelimit"
)

func TestABurstIsAllowedAndThenRefused(t *testing.T) {
	limiter := ratelimit.NewMemory(3, 60)

	for i := range 3 {
		decision, err := limiter.Allow(t.Context(), "a")
		if err != nil {
			t.Fatalf("Allow() = %v", err)
		}
		if !decision.Allowed {
			t.Fatalf("request %d of the burst was refused", i+1)
		}
	}

	decision, err := limiter.Allow(t.Context(), "a")
	if err != nil {
		t.Fatalf("Allow() = %v", err)
	}
	if decision.Allowed {
		t.Error("the request after the burst was allowed")
	}
	if decision.RetryAfter <= 0 {
		t.Error("a refusal carries no time to wait, so a client cannot obey it")
	}
}

// TestOneClientDoesNotSpendAnother is why the limiter keys on anything at all.
func TestOneClientDoesNotSpendAnother(t *testing.T) {
	limiter := ratelimit.NewMemory(1, 60)

	if decision, _ := limiter.Allow(t.Context(), "a"); !decision.Allowed {
		t.Fatal("the first request of a was refused")
	}
	if decision, _ := limiter.Allow(t.Context(), "b"); !decision.Allowed {
		t.Error("b was refused because a had spent its own allowance")
	}
}

// TestTheBucketRefills proves the limiter is a bucket and not a gate that
// closes forever.
func TestTheBucketRefills(t *testing.T) {
	limiter := ratelimit.NewMemory(1, 60)
	now := time.Now()
	ratelimit.SetClock(limiter, func() time.Time { return now })

	if decision, _ := limiter.Allow(t.Context(), "a"); !decision.Allowed {
		t.Fatal("the first request was refused")
	}
	if decision, _ := limiter.Allow(t.Context(), "a"); decision.Allowed {
		t.Fatal("the second request was allowed")
	}

	// A minute's worth of refill at sixty a minute is sixty tokens, capped at
	// the burst of one.
	now = now.Add(time.Minute)
	if decision, _ := limiter.Allow(t.Context(), "a"); !decision.Allowed {
		t.Error("the bucket did not refill")
	}
}
