package httpx_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aleogr/marketplace/internal/platform/i18n"
	"github.com/aleogr/marketplace/internal/tenancy"
)

// onConsole is a request as it reaches the routes on the console's host,
// which belongs to no marketplace (F15 spec, D1).
func onConsole(r *http.Request) *http.Request {
	ctx := tenancy.WithResolution(r.Context(), tenancy.Resolution{Kind: tenancy.ConsoleHost})
	return r.WithContext(i18n.WithLanguage(ctx, "pt-BR"))
}

// The console's host serves no marketplace: none of a store's pages is
// found there, and a form posted to one reaches neither the hasher nor the
// database (identitySite has none, so reaching it would panic).
func TestTheConsoleHostServesNoMarketplacePage(t *testing.T) {
	form := "name=Leitora&email=a%40example.test&password=correct+horse+battery"
	for _, route := range []struct{ method, path string }{
		{http.MethodGet, "/"}, {http.MethodGet, "/signup"}, {http.MethodPost, "/signup"},
		{http.MethodGet, "/verify"}, {http.MethodPost, "/verify"},
		{http.MethodGet, "/verify/resend"}, {http.MethodPost, "/verify/resend"},
		{http.MethodGet, "/account/password"}, {http.MethodPost, "/account/password"},
		{http.MethodGet, "/preview.png"},
	} {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			request := httptest.NewRequestWithContext(t.Context(), route.method, route.path, strings.NewReader(form))
			request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			recorder := httptest.NewRecorder()
			identitySite(t).ServeHTTP(recorder, onConsole(request))

			if recorder.Code != http.StatusNotFound {
				t.Errorf("status = %d, want %d", recorder.Code, http.StatusNotFound)
			}
		})
	}
}

// Whatever the deployment's setting, the console is never a page for a
// search engine to list; crawling stays allowed, so a crawler can read the
// header that refuses the listing (docs/requirements.md, section 7.1).
func TestTheConsoleHostIsNeverIndexable(t *testing.T) {
	for _, path := range []string{"/robots.txt", "/"} {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil)
		recorder := httptest.NewRecorder()
		identitySite(t).ServeHTTP(recorder, onConsole(request))

		if got := recorder.Header().Get("X-Robots-Tag"); got != "noindex, nofollow" {
			t.Errorf("%s: X-Robots-Tag = %q, want noindex, nofollow", path, got)
		}
	}
}
