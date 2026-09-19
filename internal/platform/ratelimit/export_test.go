package ratelimit

import "time"

// SetClock lets a test move time without waiting for it.
func SetClock(m *Memory, now func() time.Time) { m.now = now }

// SetDatabaseClock is the same for the database-backed limiter.
func SetDatabaseClock(d *Database, now func() time.Time) { d.now = now }
