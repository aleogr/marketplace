package httpx_test

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/aleogr/marketplace/internal/platform/httpx"
	"github.com/aleogr/marketplace/internal/platform/i18n"
)

// routes returns the site's routes, which every test here exercises through.
func routes(t *testing.T, database httpx.Database) http.Handler {
	t.Helper()

	catalogue, err := i18n.Load()
	if err != nil {
		t.Fatalf("i18n.Load() = %v", err)
	}
	return httpx.NewSite(database, catalogue).Handler()
}

func TestHealthzReportsTheRunningBuild(t *testing.T) {
	recorder := httptest.NewRecorder()
	routes(t, nil).ServeHTTP(recorder, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/health", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if got := recorder.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}

	var body struct {
		Status  string `json:"status"`
		Version string `json:"version"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not JSON: %v\nbody: %s", err, recorder.Body.String())
	}
	if body.Status != "ok" {
		t.Errorf("status = %q, want %q", body.Status, "ok")
	}
	if body.Version == "" {
		t.Error("the response carries no build identifier")
	}
}

func TestHandlerRefusesAnUnknownPath(t *testing.T) {
	recorder := httptest.NewRecorder()
	routes(t, nil).ServeHTTP(recorder, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/no-such-page", nil))

	if recorder.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
}

// listen opens a listener on a port the operating system chooses.
func listen(t *testing.T) net.Listener {
	t.Helper()

	var listenConfig net.ListenConfig
	ln, err := listenConfig.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("cannot listen: %v", err)
	}
	return ln
}

// get performs a GET bounded by the test's context.
func get(t *testing.T, url string) (*http.Response, error) {
	t.Helper()

	request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("cannot build the request: %v", err)
	}
	return http.DefaultClient.Do(request)
}

func TestServeStopsWhenTheContextIsCancelled(t *testing.T) {
	ln := listen(t)
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() { done <- httpx.Serve(ctx, ln, routes(t, nil), time.Second) }()

	response, err := get(t, "http://"+ln.Addr().String()+"/health")
	if err != nil {
		t.Fatalf("request before shutdown failed: %v", err)
	}
	response.Body.Close()

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Serve() returned %v, want nil", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Serve() did not return after the context was cancelled")
	}

	if response, err := get(t, "http://"+ln.Addr().String()+"/health"); err == nil {
		response.Body.Close()
		t.Error("the server still answers after shutdown")
	}
}

// Cloud Run sends SIGTERM and then waits. A shutdown that drops the requests
// already in flight turns every deployment into a handful of failed responses.
func TestServeLetsAnInFlightRequestFinish(t *testing.T) {
	ln := listen(t)
	ctx, cancel := context.WithCancel(context.Background())

	arrived, release := make(chan struct{}), make(chan struct{})
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(arrived)
		<-release
		w.WriteHeader(http.StatusTeapot)
	})

	done := make(chan error, 1)
	go func() { done <- httpx.Serve(ctx, ln, handler, 5*time.Second) }()

	// The status travels back, not the response: the body is closed here, where
	// it was opened.
	statuses := make(chan int, 1)
	go func() {
		response, err := get(t, "http://"+ln.Addr().String()+"/slow")
		if err != nil {
			statuses <- 0
			return
		}
		defer response.Body.Close()
		statuses <- response.StatusCode
	}()

	<-arrived
	cancel()
	close(release)

	select {
	case status := <-statuses:
		if status == 0 {
			t.Fatal("the in-flight request was dropped by the shutdown")
		}
		if status != http.StatusTeapot {
			t.Errorf("status = %d, want %d", status, http.StatusTeapot)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the in-flight request never completed")
	}

	if err := <-done; err != nil {
		t.Errorf("Serve() returned %v, want nil", err)
	}
}

func TestServeReportsAListenerItCannotUse(t *testing.T) {
	ln := listen(t)
	ln.Close()

	err := httpx.Serve(context.Background(), ln, routes(t, nil), time.Second)
	if err == nil {
		t.Fatal("Serve() returned nil for a closed listener, want an error")
	}
	if errors.Is(err, http.ErrServerClosed) {
		t.Error("Serve() reported an orderly shutdown for a broken listener")
	}
}

// database answers a health check the way a pool would.
type database struct{ err error }

func (d database) Ping(context.Context) error { return d.err }

func TestHealthReportsTheDatabase(t *testing.T) {
	t.Parallel()

	for name, testCase := range map[string]struct {
		database   httpx.Database
		wantStatus int
		wantBody   string
	}{
		"none configured": {
			database:   nil,
			wantStatus: http.StatusOK,
			wantBody:   "not configured",
		},
		"reachable": {
			database:   database{},
			wantStatus: http.StatusOK,
			wantBody:   "ok",
		},
		// A process that cannot reach its database is not ready to serve, and
		// the 503 is what makes a deployment fail instead of a visitor.
		"unreachable": {
			database:   database{err: errors.New("connection refused")},
			wantStatus: http.StatusServiceUnavailable,
			wantBody:   "unreachable",
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			recorder := httptest.NewRecorder()
			routes(t, testCase.database).ServeHTTP(recorder,
				httptest.NewRequestWithContext(t.Context(), http.MethodGet, httpx.HealthPath, nil))

			if recorder.Code != testCase.wantStatus {
				t.Errorf("status = %d, want %d", recorder.Code, testCase.wantStatus)
			}

			var body map[string]string
			if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
				t.Fatalf("cannot read the body %q: %v", recorder.Body.String(), err)
			}
			if body["database"] != testCase.wantBody {
				t.Errorf("database = %q, want %q", body["database"], testCase.wantBody)
			}
		})
	}
}
