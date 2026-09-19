package ratelimit

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"time"
)

// Subject names what a request is limited as: the client, and what it is
// asking to do.
type Subject func(r *http.Request) string

// Refused is what a limiter's middleware writes when the limit is reached.
// The page belongs to the HTTP layer, so it is passed in rather than imported.
type Refused func(w http.ResponseWriter, r *http.Request, retryAfter time.Duration)

// Limit refuses a request whose subject has done this too often.
//
// A limiter that fails is not a limiter that allows: an unreachable counter
// closes the door, because the endpoints this guards are the ones worth
// guarding, and an attacker who can make the database unavailable would
// otherwise also unlock them. The failure is logged, so a limit that is closing
// the door for the wrong reason is visible rather than silent.
func Limit(limiter Limiter, subject Subject, refused Refused, log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			decision, err := limiter.Allow(r.Context(), subject(r))
			switch {
			case err != nil:
				log.ErrorContext(r.Context(), "the rate limiter could not decide",
					"error", err, "path", r.URL.Path)
				refused(w, r, 0)
				return
			case !decision.Allowed:
				refused(w, r, decision.RetryAfter)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RetryAfter writes the header a refused client should obey, when there is a
// meaningful answer to give.
func RetryAfter(w http.ResponseWriter, wait time.Duration) {
	if wait <= 0 {
		return
	}
	w.Header().Set("Retry-After", strconv.Itoa(int(wait.Seconds())))
}

// Never is a limiter that allows everything. It is what a process running
// without a database uses in place of the database-backed limiter, so that the
// pipeline has the same shape in both modes.
type Never struct{}

func (Never) Allow(context.Context, string) (Decision, error) { return allowed, nil }
