package httpx_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// securityRoutes are the pages of the security area, by method.
var securityRoutes = []struct{ method, path string }{
	{http.MethodGet, "/account/security"},
	{http.MethodGet, "/account/security/app"},
	{http.MethodPost, "/account/security/app"},
	{http.MethodPost, "/account/security/remove"},
	{http.MethodPost, "/account/security/recovery"},
	{http.MethodGet, "/account/security/email"},
	{http.MethodPost, "/account/security/email/send"},
	{http.MethodPost, "/account/security/email"},
	{http.MethodGet, "/account/verify?for=factors&next=%2Faccount%2Fsecurity"},
	{http.MethodPost, "/account/verify"},
	{http.MethodPost, "/account/verify/email"},
}

// The security area is a signed-in area: anybody else is sent to sign in,
// before the service is asked anything (identitySite has no database, so
// reaching it would panic).
func TestTheSecurityPagesSendASignedOutVisitorToSignIn(t *testing.T) {
	for _, route := range securityRoutes {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			request := httptest.NewRequestWithContext(t.Context(), route.method, route.path, strings.NewReader(""))
			request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			recorder := httptest.NewRecorder()
			identitySite(t).ServeHTTP(recorder, inMarketplace(request))
			if recorder.Code != http.StatusSeeOther || recorder.Header().Get("Location") != "/pt-BR/signin" {
				t.Errorf("status %d, Location %q; want %d to /pt-BR/signin",
					recorder.Code, recorder.Header().Get("Location"), http.StatusSeeOther)
			}
		})
	}
}

// The platform's own host has no accounts, and so no security area.
func TestTheSecurityPagesAreNotFoundOnThePlatformHost(t *testing.T) {
	for _, route := range securityRoutes {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			request := httptest.NewRequestWithContext(t.Context(), route.method, route.path, strings.NewReader(""))
			request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			recorder := httptest.NewRecorder()
			identitySite(t).ServeHTTP(recorder, signedIn(onPlatform(request)))
			if recorder.Code != http.StatusNotFound {
				t.Errorf("status = %d, want %d", recorder.Code, http.StatusNotFound)
			}
		})
	}
}

// A signed-in header links to the security page, in the page's language.
func TestTheHeaderLinksToTheSecurityPage(t *testing.T) {
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/account/password", nil)
	recorder := httptest.NewRecorder()
	identitySite(t).ServeHTTP(recorder, signedIn(inMarketplace(request)))
	body := recorder.Body.String()
	if !strings.Contains(body, `href="/pt-BR/account/security"`) || !strings.Contains(body, ">Segurança</a>") {
		t.Fatalf("the header does not link to the security page: %s", body)
	}
}
