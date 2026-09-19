package ratelimit

import (
	"context"
	"math"
	"sync"
	"time"
)

// Memory is a token bucket per key, held in one process.
//
// A bucket refills continuously rather than resetting on a boundary, so a
// client that spends its allowance does not get a full one again at the top of
// the minute — the trick that makes a fixed window let through twice the limit
// across its edge.
type Memory struct {
	// burst is how many requests may arrive at once, and the size of a full
	// bucket.
	burst float64
	// refill is tokens per second.
	refill float64
	// idle is how long a bucket with nothing in it is kept before it is
	// forgotten. Without this the map grows with every address ever seen.
	idle time.Duration

	now func() time.Time

	mu      sync.Mutex
	buckets map[string]*bucket
}

type bucket struct {
	tokens float64
	seen   time.Time
}

// NewMemory returns a limiter allowing burst requests at once and per minute
// thereafter.
func NewMemory(burst int, perMinute int) *Memory {
	return &Memory{
		burst:   float64(burst),
		refill:  float64(perMinute) / 60,
		idle:    10 * time.Minute,
		now:     time.Now,
		buckets: map[string]*bucket{},
	}
}

// Allow spends one token for key.
func (m *Memory) Allow(_ context.Context, key string) (Decision, error) {
	now := m.now()

	m.mu.Lock()
	defer m.mu.Unlock()

	m.forgetIdle(now)

	b, ok := m.buckets[key]
	if !ok {
		// A key nobody has seen starts full. Its zero `seen` also makes the
		// refill below a no-op, since the bucket is already at the brim.
		b = &bucket{tokens: m.burst, seen: now}
		m.buckets[key] = b
	}

	b.tokens = math.Min(m.burst, b.tokens+now.Sub(b.seen).Seconds()*m.refill)
	b.seen = now

	if b.tokens < 1 {
		// How long until one whole token exists again.
		wait := time.Duration((1 - b.tokens) / m.refill * float64(time.Second))
		return Decision{RetryAfter: wait.Round(time.Second) + time.Second}, nil
	}

	b.tokens--
	return allowed, nil
}

// forgetIdle drops the buckets nobody has touched for a while. It runs under
// the same lock as Allow, and only walks the map when it has grown enough to
// be worth walking.
func (m *Memory) forgetIdle(now time.Time) {
	const worthWalking = 1024
	if len(m.buckets) < worthWalking {
		return
	}
	for key, b := range m.buckets {
		if now.Sub(b.seen) > m.idle {
			delete(m.buckets, key)
		}
	}
}
