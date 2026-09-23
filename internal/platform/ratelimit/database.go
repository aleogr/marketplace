package ratelimit

import (
	"context"
	"fmt"
	"regexp"
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
//
// Every limiter has a name, and its rows are its own. The table is shared by
// limits with windows of minutes and of hours, and a sweep that forgot every
// row older than its own window would erase the live counters of a longer
// one: an hourly limit reset every ten minutes by its neighbour is not an
// hourly limit.
type Database struct {
	db     Execer
	name   string
	limit  int
	window time.Duration

	now func() time.Time
}

// validName is what a limiter may be called: it is the prefix of every row it
// writes and of the pattern its sweep matches, so it holds nothing a LIKE
// pattern would read as a wildcard.
var validName = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// NewDatabase returns a limiter called name, allowing limit attempts per window
// for a key. One name is one limit: two limiters sharing a name must share the
// window too, which is what two instances of the same limit do.
//
// A name that is not lower-case letters, digits and hyphens is a programming
// error, and it panics at start-up rather than miscounting later.
func NewDatabase(db Execer, name string, limit int, window time.Duration) *Database {
	if !validName.MatchString(name) {
		panic(fmt.Sprintf("ratelimit: %q is not a limiter name", name))
	}
	return &Database{db: db, name: name, limit: limit, window: window, now: time.Now}
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
	`, d.name+":"+key, start).Scan(&hits)
	if err != nil {
		return Decision{}, fmt.Errorf("cannot count the attempt: %w", err)
	}

	// The windows of this limiter that have passed are forgotten by whoever
	// opens a new one, and only this limiter's: another limiter's rows may
	// belong to a longer window that is still open. There is no scheduler in
	// this phase and a table of expired counters would otherwise grow forever;
	// hanging the sweep on the first hit of a window makes it rare without
	// needing a coin toss to make it rare.
	if hits == 1 {
		if _, err := d.db.Exec(ctx,
			"DELETE FROM rate_limit WHERE subject LIKE $1 AND window_start < $2",
			d.name+":%", start.Add(-d.window)); err != nil {
			return Decision{}, fmt.Errorf("cannot forget the windows that have passed: %w", err)
		}
	}

	if hits > d.limit {
		return Decision{RetryAfter: start.Add(d.window).Sub(now).Round(time.Second) + time.Second}, nil
	}
	return allowed, nil
}
