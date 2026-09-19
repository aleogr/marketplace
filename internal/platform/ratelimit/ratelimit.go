// Package ratelimit bounds how often one client may do something.
//
// Two tiers, because they answer different questions (docs/roadmap.md, F7).
// General traffic is bounded per instance, in memory, which costs nothing and
// is enough to blunt a flood. The sensitive endpoints — signing in, signing up,
// recovering a password, and later checkout — are bounded in the database,
// because Cloud Run runs several instances at once and a counter held in one
// process would let an attacker have the limit once per instance.
package ratelimit

import (
	"context"
	"time"
)

// Decision is the answer to "may this one proceed".
type Decision struct {
	Allowed bool
	// RetryAfter is how long the caller should wait, and is only meaningful
	// when Allowed is false.
	RetryAfter time.Duration
}

var allowed = Decision{Allowed: true}

// Limiter decides whether a key may act again.
//
// An error is not a decision: a limiter that cannot reach its store says so,
// and the caller decides whether that failure closes the door or opens it.
type Limiter interface {
	Allow(ctx context.Context, key string) (Decision, error)
}
