package httpx_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aleogr/marketplace/internal/platform/httpx"
)

// TestTheClientCannotChooseItsOwnAddress is the property the limit depends on.
//
// Everything to the left of what the infrastructure appended is written by
// whoever is being limited, so a client that forges an address must still be
// counted as itself.
func TestTheClientCannotChooseItsOwnAddress(t *testing.T) {
	const (
		client   = "203.0.113.7"
		frontEnd = "35.191.0.1"
	)

	for name, forwarded := range map[string]string{
		"nothing forged":   client + ", " + frontEnd,
		"one forged entry": "1.2.3.4, " + client + ", " + frontEnd,
		"a whole chain":    "1.2.3.4, 5.6.7.8, 9.10.11.12, " + client + ", " + frontEnd,
		"nonsense forged":  "not-an-address, " + client + ", " + frontEnd,
	} {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "https://marketplace.example/", nil)
		request.Header.Set("X-Forwarded-For", forwarded)

		if got := httpx.ClientIP(request, 2); got != client {
			t.Errorf("%s: ClientIP() = %q, want %q", name, got, client)
		}
	}
}

// TestASplitHeaderIsOneChain: a client may send the header several times
// rather than as one comma-separated list, and a reader that took only the
// first or the last header would be reading a chain of its choosing.
func TestASplitHeaderIsOneChain(t *testing.T) {
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "https://marketplace.example/", nil)
	request.Header.Add("X-Forwarded-For", "1.2.3.4")
	request.Header.Add("X-Forwarded-For", "203.0.113.7, 35.191.0.1")

	if got := httpx.ClientIP(request, 2); got != "203.0.113.7" {
		t.Errorf("ClientIP() = %q, want 203.0.113.7", got)
	}
}

// TestWithoutTheHeaderThePeerIsTheClient covers a local process and the
// end-to-end suite, where nothing sits in front at all.
func TestWithoutTheHeaderThePeerIsTheClient(t *testing.T) {
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "http://localhost:8080/", nil)
	request.RemoteAddr = "198.51.100.23:54321"

	if got := httpx.ClientIP(request, 2); got != "198.51.100.23" {
		t.Errorf("ClientIP() = %q, want 198.51.100.23", got)
	}
}

// TestTooFewEntriesFallsBackToThePeer is the shape of being wrong that matters.
//
// If the hop count is larger than the chain — the infrastructure adds fewer
// entries than configured — the answer is the peer, which is the front end.
// Everyone is then limited together, which is heavy-handed; the other
// direction, trusting an entry the client wrote, would limit nobody at all.
func TestTooFewEntriesFallsBackToThePeer(t *testing.T) {
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "https://marketplace.example/", nil)
	request.Header.Set("X-Forwarded-For", "1.2.3.4")
	request.RemoteAddr = "35.191.0.1:443"

	if got := httpx.ClientIP(request, 2); got != "35.191.0.1" {
		t.Errorf("ClientIP() = %q, want the peer 35.191.0.1", got)
	}
}

func TestTheOriginReachesTheHandler(t *testing.T) {
	var origin httpx.Origin
	handler := httpx.Origins(2)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		origin, _ = httpx.OriginFrom(r.Context())
	}))

	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "https://marketplace.example/", nil)
	request.Header.Set("X-Forwarded-For", "203.0.113.7, 35.191.0.1")
	request.Header.Set("User-Agent", "Mozilla/5.0 (a browser)")

	handler.ServeHTTP(httptest.NewRecorder(), request)

	if origin.IP != "203.0.113.7" {
		t.Errorf("IP = %q, want 203.0.113.7", origin.IP)
	}
	if origin.UserAgent != "Mozilla/5.0 (a browser)" {
		t.Errorf("UserAgent = %q", origin.UserAgent)
	}
}
