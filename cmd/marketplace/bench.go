package main

import (
	"context"
	"log/slog"
	"sort"
	"time"

	"github.com/aleogr/marketplace/internal/identity"
)

// benchBudget is how long one password verification may take on the service's
// CPU. A sign-in costs one; 250 ms is felt by nobody and costs an attacker
// with the database 250 ms per guess per core.
const benchBudget = 250 * time.Millisecond

type benchResult struct {
	Params identity.Params
	Median time.Duration
}

// benchPassword measures argon2id on this machine, for the candidates OWASP
// lists as equivalent, and logs the choice (spec, D5). It is run once, as the
// lab's migration job with this argument, because that job runs on the same
// CPU as the service (docs/infrastructure.md). The chosen line becomes
// identity.Current.
func benchPassword(ctx context.Context, log *slog.Logger) error {
	candidates := []identity.Params{
		{Memory: 19456, Time: 2, Threads: 1, KeyLen: 32, SaltLen: 16},
		{Memory: 47104, Time: 1, Threads: 1, KeyLen: 32, SaltLen: 16},
		{Memory: 47104, Time: 2, Threads: 1, KeyLen: 32, SaltLen: 16},
		{Memory: 65536, Time: 2, Threads: 1, KeyLen: 32, SaltLen: 16},
		{Memory: 65536, Time: 3, Threads: 1, KeyLen: 32, SaltLen: 16},
	}
	var results []benchResult
	for _, p := range candidates {
		h := identity.NewHasher(p, 1)
		var runs []time.Duration
		for range 9 {
			start := time.Now()
			if _, err := h.Hash(ctx, "benchmark passphrase of ordinary length"); err != nil {
				return err
			}
			runs = append(runs, time.Since(start))
		}
		sort.Slice(runs, func(i, j int) bool { return runs[i] < runs[j] })
		r := benchResult{Params: p, Median: runs[len(runs)/2]}
		results = append(results, r)
		log.InfoContext(ctx, "argon2id", "memory_kib", p.Memory, "time", p.Time, "median_ms", r.Median.Milliseconds())
	}
	chosen := choose(results, benchBudget)
	log.InfoContext(ctx, "argon2id chosen", "memory_kib", chosen.Memory, "time", chosen.Time,
		"budget_ms", benchBudget.Milliseconds())
	return nil
}

// choose returns the most expensive candidate, by memory times time, whose
// median stayed within budget, or the OWASP floor when none did. Every
// candidate above is within identity's decode ceilings (256 MiB, t=10;
// internal/identity/password.go), so a chosen value is always a value the
// service can also read back.
func choose(results []benchResult, budget time.Duration) identity.Params {
	best := identity.Floor
	var bestCost uint64
	for _, r := range results {
		cost := uint64(r.Params.Memory) * uint64(r.Params.Time)
		if r.Median <= budget && cost > bestCost {
			best, bestCost = r.Params, cost
		}
	}
	return best
}
