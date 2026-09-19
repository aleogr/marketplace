package httpx_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aleogr/marketplace/internal/platform/httpx"
)

// served returns the headers of a response written through the middleware.
//
// The headers, not the response: nothing here reads a body, and handing one
// out would leave every caller with something to close.
func served(t *testing.T, indexable bool, request *http.Request) http.Header {
	t.Helper()

	var nonce string
	handler := httpx.Secure(indexable)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nonce = httpx.Nonce(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	header := recorder.Header()
	if nonce == "" {
		t.Error("the handler was given no nonce, so no template can mark a script with one")
	}
	if policy := header.Get("Content-Security-Policy"); !strings.Contains(policy, "'nonce-"+nonce+"'") {
		t.Errorf("the policy does not carry the nonce the handler was given: %q", policy)
	}
	return header
}

func TestEveryResponseCarriesTheHeaders(t *testing.T) {
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "https://marketplace.example/", nil)
	request.Header.Set("X-Forwarded-Proto", "https")

	header := served(t, true, request)

	for name, want := range map[string]string{
		"X-Content-Type-Options":    "nosniff",
		"Referrer-Policy":           "strict-origin-when-cross-origin",
		"Strict-Transport-Security": "max-age=31536000; includeSubDomains",
	} {
		if got := header.Get(name); got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}

	policy := header.Get("Content-Security-Policy")
	for _, directive := range []string{
		"default-src 'self'", "frame-ancestors 'none'", "object-src 'none'",
		"base-uri 'none'", "form-action 'self'",
	} {
		if !strings.Contains(policy, directive) {
			t.Errorf("the policy is missing %q: %s", directive, policy)
		}
	}
	// 'unsafe-inline' would undo the nonce entirely: a browser that
	// understands nonces ignores it, but one that does not would run anything.
	if strings.Contains(policy, "unsafe-inline") || strings.Contains(policy, "unsafe-eval") {
		t.Errorf("the policy allows inline or evaluated script: %s", policy)
	}

	if robots := header.Get("X-Robots-Tag"); robots != "" {
		t.Errorf("an indexable deployment sent %q", robots)
	}
}

// TestAnUnindexableDeploymentSaysSo covers the lab, and every environment that
// is not the public one (docs/requirements.md, section 7.1).
func TestAnUnindexableDeploymentSaysSo(t *testing.T) {
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "https://marketplace.example/", nil)
	header := served(t, false, request)

	if got := header.Get("X-Robots-Tag"); got != "noindex, nofollow" {
		t.Errorf("X-Robots-Tag = %q, want noindex, nofollow", got)
	}
}

// TestStrictTransportSecurityIsNotClaimedOverPlainHTTP: the header is a promise
// about an address, and a process reached over plain HTTP is in no position to
// make it.
func TestStrictTransportSecurityIsNotClaimedOverPlainHTTP(t *testing.T) {
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "http://localhost:8080/", nil)
	header := served(t, true, request)

	if got := header.Get("Strict-Transport-Security"); got != "" {
		t.Errorf("Strict-Transport-Security = %q over plain HTTP, want nothing", got)
	}
}

// TestTheNonceIsNewOnEveryResponse is what makes a nonce worth having: one
// reused across responses can be read from one page and used in another.
func TestTheNonceIsNewOnEveryResponse(t *testing.T) {
	seen := map[string]bool{}
	handler := httpx.Secure(true)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		nonce := httpx.Nonce(r.Context())
		if seen[nonce] {
			t.Errorf("the nonce %q was served twice", nonce)
		}
		seen[nonce] = true
	}))

	for range 100 {
		handler.ServeHTTP(httptest.NewRecorder(),
			httptest.NewRequestWithContext(t.Context(), http.MethodGet, "https://marketplace.example/", nil))
	}
	if len(seen) != 100 {
		t.Errorf("100 responses carried %d distinct nonces", len(seen))
	}
}
