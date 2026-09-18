package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

// syncBuffer lets the test read the log while the server goroutine writes it.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func (b *syncBuffer) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Len()
}

func lookupFrom(vars map[string]string) func(string) (string, bool) {
	return func(key string) (string, bool) {
		value, ok := vars[key]
		return value, ok
	}
}

// freePort returns a port nothing is listening on.
func freePort(t *testing.T) int {
	t.Helper()

	var listenConfig net.ListenConfig
	ln, err := listenConfig.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("cannot listen: %v", err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

// findEntry returns the first log entry whose message matches.
func findEntry(t *testing.T, logged string, message string) map[string]any {
	t.Helper()

	for _, line := range strings.Split(strings.TrimSpace(logged), "\n") {
		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("log line is not JSON: %v\nline: %s", err, line)
		}
		if entry["message"] == message {
			return entry
		}
	}
	t.Fatalf("no entry with message %q in:\n%s", message, logged)
	return nil
}

// A configuration the process cannot read must stop it, and the message must
// name the variable: an operator reading "invalid configuration" alone has to
// guess which of the declared variables is wrong.
func TestRunRefusesToStartOnAnUnreadableValue(t *testing.T) {
	var stdout syncBuffer

	err := run(context.Background(), lookupFrom(map[string]string{"INDEXABLE": "ture"}), &stdout)

	if err == nil {
		t.Fatal("run() started with INDEXABLE=ture, want an error")
	}
	if !strings.Contains(err.Error(), "INDEXABLE") {
		t.Errorf("error %q does not name the variable", err)
	}
	if stdout.Len() != 0 {
		t.Errorf("wrote %q before failing, want nothing", stdout.String())
	}
}

// Which build is running and whether it may be indexed are the two facts that
// are expensive to discover any other way, so both are logged on every start
// (docs/requirements.md, sections 7.1 and 27).
func TestRunLogsTheBuildAndTheIndexingModeOnEveryStart(t *testing.T) {
	var stdout syncBuffer
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- run(ctx, lookupFrom(map[string]string{
			"PORT":      fmt.Sprint(freePort(t)),
			"INDEXABLE": "false",
		}), &stdout)
	}()

	waitFor(t, func() bool { return strings.Contains(stdout.String(), "server started") })
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("run() returned %v, want nil", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("run() did not return after the context was cancelled")
	}

	entry := findEntry(t, stdout.String(), "server started")
	if entry["version"] == "" || entry["version"] == nil {
		t.Error("the start-up entry carries no build identifier")
	}
	if entry["indexable"] != false {
		t.Errorf(`entry["indexable"] = %v, want false`, entry["indexable"])
	}
}

func TestRunServesTheHealthCheck(t *testing.T) {
	var stdout syncBuffer
	port := freePort(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- run(ctx, lookupFrom(map[string]string{"PORT": fmt.Sprint(port)}), &stdout)
	}()
	waitFor(t, func() bool { return strings.Contains(stdout.String(), "server started") })

	request, err := http.NewRequestWithContext(t.Context(), http.MethodGet,
		fmt.Sprintf("http://127.0.0.1:%d/healthz", port), nil)
	if err != nil {
		t.Fatalf("cannot build the request: %v", err)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("health check failed: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != 200 {
		t.Errorf("status = %d, want 200", response.StatusCode)
	}

	cancel()
	if err := <-done; err != nil {
		t.Errorf("run() returned %v, want nil", err)
	}
}

// waitFor polls until condition holds or the test's patience runs out.
func waitFor(t *testing.T, condition func() bool) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("the condition never held")
}
