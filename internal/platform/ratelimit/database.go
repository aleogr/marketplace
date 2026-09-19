package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Execer is the part of the connection pool this limiter uses.
type Execer interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// Database counts attempts in PostgreSQL, so that the limit is the limit
// however many instances are serving.
//
// It is a fixed window rather than a token bucket: one statement, one row, no
// read-then-write race between instances. The edge of a window lets through
// somewhat more than the limit over a short span, which is the cost, and for
// "five sign-in attempts a minute" it is a cost worth paying to keep the
// counter a single atomic upsert.
type Database struct {
	db     Execer
	limit  int
	window time.Duration

	now func() time.Time
}

// NewDatabase returns a limiter allowing limit attempts per window for a key.
func NewDatabase(db Execer, limit int, window time.Duration) *Database {
	return &Database{db: db, limit: limit, window: window, now: time.Now}
}

// Allow counts one attempt for key and reports whether it is within the limit.
func (d *Database) Allow(ctx context.Context, key string) (Decision, error) {
	now := d.now().UTC()
	// The window a moment belongs to, so that every instance agrees on which
	// row to write without any of them coordinating.
	start := now.Truncate(d.window)

	var hits int
	err := d.db.QueryRow(ctx, `
		INSERT INTO rate_limit (subject, window_start, hits)
		VALUES ($1, $2, 1)
		ON CONFLICT (subject, window_start)
		DO UPDATE SET hits = rate_limit.hits + 1
		RETURNING hits
	`, key, start).Scan(&hits)
	if err != nil {
		return Decision{}, fmt.Errorf("cannot count the attempt: %w", err)
	}

	// The windows that have passed are forgotten by whoever opens a new one.
	// There is no scheduler in this phase and a table of expired counters would
	// otherwise grow forever; hanging the sweep on the first hit of a window
	// makes it rare without needing a coin toss to make it rare.
	if hits == 1 {
		if _, err := d.db.Exec(ctx,
			"DELETE FROM rate_limit WHERE window_start < $1", start.Add(-d.window)); err != nil {
			return Decision{}, fmt.Errorf("cannot forget the windows that have passed: %w", err)
		}
	}

	if hits > d.limit {
		return Decision{RetryAfter: start.Add(d.window).Sub(now).Round(time.Second) + time.Second}, nil
	}
	return allowed, nil
}
