package httpx_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aleogr/marketplace/internal/platform/httpx"
)

// The site's one script is served from this origin, as JavaScript, and asked
// for again on every page: while the build is the same the answer is "not
// modified", and a new build's script is never mistaken for the old one.
func TestTheWebAuthnScriptIsServedAndRevalidated(t *testing.T) {
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, httpx.ScriptPath, nil)
	recorder := httptest.NewRecorder()
	identitySite(t).ServeHTTP(recorder, request)
	etag := recorder.Header().Get("ETag")
	if recorder.Code != http.StatusOK || recorder.Header().Get("Content-Type") != "text/javascript; charset=utf-8" ||
		recorder.Header().Get("Cache-Control") != "no-cache" || etag == "" ||
		!strings.Contains(recorder.Body.String(), "navigator.credentials") {
		t.Fatalf("status %d, headers %v", recorder.Code, recorder.Header())
	}

	again := httptest.NewRequestWithContext(t.Context(), http.MethodGet, httpx.ScriptPath, nil)
	again.Header.Set("If-None-Match", etag)
	recorder = httptest.NewRecorder()
	identitySite(t).ServeHTTP(recorder, again)
	if recorder.Code != http.StatusNotModified {
		t.Fatalf("a second request with the tag = %d, want %d", recorder.Code, http.StatusNotModified)
	}
}
