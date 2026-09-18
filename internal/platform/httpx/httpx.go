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
	"net"
	"net/http"
	"time"

	"github.com/aleogr/marketplace/internal/platform/version"
)

// readHeaderTimeout bounds how long a client may take to send its headers, so
// that an idle connection cannot hold a handler slot open indefinitely.
const readHeaderTimeout = 10 * time.Second

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

// Handler returns the routes served by the process.
func Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET "+HealthPath, health)
	return mux
}

func health(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"version": version.String(),
	})
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
