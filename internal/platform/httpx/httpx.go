// Package httpx assembles the HTTP server.
//
// In this phase it serves only the health check. The request pipeline of
// docs/design.md, section 2.3 — marketplace resolution, language, session,
// tenant context and the security middlewares — is mounted here by the
// deliveries that introduce it.
package httpx

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/aleogr/marketplace/internal/platform/ratelimit"
	"github.com/aleogr/marketplace/internal/platform/version"
)

// readHeaderTimeout bounds how long a client may take to send its headers, so
// that an idle connection cannot hold a handler slot open indefinitely.
const readHeaderTimeout = 10 * time.Second

// LabOrigin is where the laboratory's index is served from, and the one origin
// allowed to read HealthPath from a reader's browser (aleogr/lab). Without the
// answer naming it, the same-origin policy refuses that read.
//
// A production deployment answers with it too, and that is harmless: the header
// grants the right to read a status, a version and whether the database
// replies, to one page, and grants nothing else to anyone.
const LabOrigin = "https://lab.aleogr.dev"

// HealthPath is where the process answers a health check.
//
// It is `/health` and must not become `/healthz`, which is the conventional
// name and the one this project used until it was deployed. On Cloud Run,
// Google's front end answers the literal path `/healthz` with its own 404 page
// and the request never reaches the container: it arrives with no
// `x-cloud-trace-context`, and neighbouring paths — `/healthz/`, `/healthZ`,
// `/livez`, `/readyz` — all pass through untouched, so this is specific to that
// one string rather than a rule about paths ending in `z`. Nothing in Google's
// container contract mentions it, and nothing in this repository could have
// found it before the first deployment, because a local process serves
// `/healthz` perfectly well.
const HealthPath = "/health"

// Database is the part of the connection pool the health check needs: whether
// the database answers. Taking an interface rather than the pool keeps this
// package free of a database dependency it would otherwise carry into every
// test that builds a handler.
type Database interface {
	Ping(ctx context.Context) error
}

// Pipeline is what a request passes through before it reaches a handler.
//
// The order is the one in docs/design.md, section 2.3, and each step is in its
// place for a reason:
//
//   - the origin first, because everything after it keys on the address;
//   - the security headers next, so that they are on the response whatever
//     happens later, including a refusal;
//   - the general rate limit, which turns a flood away before it costs a
//     database round trip;
//   - marketplace resolution, which is mounted by the caller because it needs
//     the database;
//   - CSRF, which is last, because a forged request should be counted by the
//     limiter like any other.
//
// The health check is exempt from the limit and from resolution: it is called
// by the platform's probes and by every deployment, from an address that is
// not a visitor's and for a host no marketplace claims.
type Pipeline struct {
	// Indexable is the deployment's own setting.
	Indexable bool
	// ProxyHops is how many entries Google's front end adds to
	// `X-Forwarded-For` (internal/platform/httpx.ClientIP).
	ProxyHops int
	// General bounds ordinary traffic. Nil mounts no general limit.
	General ratelimit.Limiter
	// Pages writes the refusals: they are the only part of the pipeline a
	// person reads, so they come from the catalogue like every other text.
	Pages Pages
	// Callbacks are the addresses a machine calls rather than a browser: the
	// e-mail provider's webhook, and whatever later deliveries add. They are
	// outside CSRF, for the same reason the task endpoint is — there is no
	// page to mint a token on — and inside the general rate limit, because a
	// provider that posts is a caller from the internet like any other.
	Callbacks []string
	// Log is where a limiter that cannot decide says so.
	Log *slog.Logger
}

// Wrap mounts the pipeline around handler.
func (p Pipeline) Wrap(handler http.Handler) http.Handler {
	// The callback endpoint is outside both. It carries no cookies and is not
	// reached from a page, so a CSRF token would be a token nobody can mint;
	// what guards it is the signed token of the account calling it
	// (internal/platform/httpx.Tasks). And it is outside the limit because
	// throttling Cloud Tasks would delay work this service asked for itself,
	// while the queue's own rate is what bounds it.
	wrapped := exempt(CSRF(p.Pages), append([]string{TasksPath}, p.Callbacks...)...)(handler)

	if p.General != nil {
		byAddress := func(r *http.Request) string {
			origin, _ := OriginFrom(r.Context())
			return "general:" + origin.IP
		}
		wrapped = exempt(ratelimit.Limit(p.General, byAddress, p.Pages.Refused, p.Log),
			HealthPath, TasksPath)(wrapped)
	}

	wrapped = Secure(p.Indexable)(wrapped)
	return Origins(p.ProxyHops)(wrapped)
}

// exempt mounts middleware on everything but the paths given.
func exempt(middleware func(http.Handler) http.Handler, paths ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		wrapped := middleware(next)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			for _, path := range paths {
				if r.URL.Path == path {
					next.ServeHTTP(w, r)
					return
				}
			}
			wrapped.ServeHTTP(w, r)
		})
	}
}

// health answers what this process is and whether it can do its work.
//
// It is a readiness check, not only a liveness one: a process that cannot reach
// its database is not ready to serve, and saying so with a 503 is what makes
// the deployment fail instead of the first visitor (docs/roadmap.md, F4).
func health(database Database) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body := map[string]string{
			"status":  "ok",
			"version": version.String(),
		}
		status := http.StatusOK

		switch {
		case database == nil:
			body["database"] = "not configured"
		case database.Ping(r.Context()) != nil:
			body["status"] = "unavailable"
			body["database"] = "unreachable"
			status = http.StatusServiceUnavailable
		default:
			body["database"] = "ok"
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Access-Control-Allow-Origin", LabOrigin)
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(body)
	}
}

// Serve accepts connections on ln until ctx is cancelled, then stops accepting
// and gives the requests already in flight up to grace to finish.
//
// Cloud Run sends SIGTERM and waits before killing the instance, so a shutdown
// that dropped in-flight requests would turn every deployment into a handful of
// failed responses.
func Serve(ctx context.Context, ln net.Listener, handler http.Handler, grace time.Duration) error {
	server := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
	}

	serveErr := make(chan error, 1)
	go func() { serveErr <- server.Serve(ln) }()

	select {
	case err := <-serveErr:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
	}

	// The shutdown must outlive the context that triggered it.
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), grace)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		return err
	}
	if err := <-serveErr; err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
