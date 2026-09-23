package httpx_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/aleogr/marketplace/internal/identity"
	"github.com/aleogr/marketplace/internal/platform/httpx"
)

// A request with no session cookie carries no session and never reaches the
// service: the middleware is given none here, and would panic if it did.
func TestNoCookieMeansNoSessionAndNoDatabaseCall(t *testing.T) {
	var seen, reached bool
	handler := httpx.Sessions(nil, silent())(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		reached = true
		_, seen = httpx.SessionFrom(r.Context())
	}))
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	handler.ServeHTTP(httptest.NewRecorder(), inMarketplace(request))
	if !reached {
		t.Fatal("the request did not reach the handler")
	}
	if seen {
		t.Fatal("a request with no cookie carried a session")
	}
}

// The platform's own host has no accounts, so a session cookie sent there is
// not looked up: the middleware has no service, and would panic if it did.
func TestACookieOnThePlatformHostIsNotLookedUp(t *testing.T) {
	var seen bool
	handler := httpx.Sessions(nil, silent())(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		_, seen = httpx.SessionFrom(r.Context())
	}))
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	request.AddCookie(&http.Cookie{Name: "session", Value: "anything"})
	handler.ServeHTTP(httptest.NewRecorder(), onPlatform(request))
	if seen {
		t.Fatal("a request on the platform host carried a session")
	}
}

// Sign-in replaces the CSRF secret the browser held (sign-in CSRF, session
// fixation).
func TestRenewCSRFReplacesTheSecret(t *testing.T) {
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/signin", nil)
	request.AddCookie(&http.Cookie{Name: "csrf", Value: "planted"})
	recorder := httptest.NewRecorder()

	httpx.RenewCSRF(recorder, request)

	var renewed *http.Cookie
	for _, cookie := range recorder.Result().Cookies() {
		if cookie.Name == "csrf" {
			renewed = cookie
		}
	}
	if renewed == nil || renewed.Value == "" || renewed.Value == "planted" {
		t.Fatalf("the CSRF cookie was not renewed: %+v", renewed)
	}
	if !renewed.HttpOnly || renewed.SameSite != http.SameSiteLaxMode || renewed.Path != "/" {
		t.Errorf("the renewed cookie lost its flags: %+v", renewed)
	}
}

// An unknown address and a wrong password are answered with one sentence
// (D7). An address that is not one stops before the database, which this site
// has none of; the typed address is shown back, the password never.
func TestARefusedSignInSaysOneSentence(t *testing.T) {
	recorder := postForm(t, "/signin", url.Values{"email": {"not an address"}, "password": {"a secret phrase here"}})

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, "O e-mail ou a senha não conferem.") {
		t.Errorf("the page does not say the sign-in was refused: %s", body)
	}
	if !strings.Contains(body, `value="not an address"`) {
		t.Errorf("the page does not show the typed address back")
	}
	if strings.Contains(body, "a secret phrase here") {
		t.Errorf("the page shows the password back")
	}
}

func TestTheSignInPageSaysAConfirmedAddressCanSignIn(t *testing.T) {
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/signin?verified=1", nil)
	recorder := httptest.NewRecorder()
	identitySite(t).ServeHTTP(recorder, inMarketplace(request))

	body := recorder.Body.String()
	if recorder.Code != http.StatusOK || !strings.Contains(body, "Seu e-mail foi confirmado.") {
		t.Fatalf("status %d, the page does not confirm the address: %s", recorder.Code, body)
	}
	if !strings.Contains(body, `autocomplete="current-password"`) {
		t.Errorf("the password field does not ask for the current password")
	}
}

// accountNav returns the header's account navigation of a page, or "" when
// the page has none.
func accountNav(body string) string {
	start := strings.Index(body, `<nav class="account"`)
	if start < 0 {
		return ""
	}
	end := strings.Index(body[start:], "</nav>")
	if end < 0 {
		return body[start:]
	}
	return body[start : start+end]
}

// The header offers the two ways in to a visitor, and the name and the way out
// to an account; the platform's own pages, which have no accounts, offer
// neither.
func TestTheHeaderShowsWhoIsSignedIn(t *testing.T) {
	get := func(r *http.Request) string {
		recorder := httptest.NewRecorder()
		identitySite(t).ServeHTTP(recorder, r)
		return recorder.Body.String()
	}

	signedOut := accountNav(get(inMarketplace(
		httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))))
	for _, want := range []string{`aria-label="Conta"`, `href="/pt-BR/signin"`, `href="/pt-BR/signup"`} {
		if !strings.Contains(signedOut, want) {
			t.Errorf("the signed-out header does not carry %s: %q", want, signedOut)
		}
	}
	if strings.Contains(signedOut, "/signout") {
		t.Errorf("the signed-out header offers to sign out: %q", signedOut)
	}

	request := inMarketplace(httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	request = request.WithContext(httpx.WithSession(request.Context(),
		identity.Session{ID: "s", Account: identity.Account{Name: "Leitora"}}))
	signedIn := accountNav(get(request))
	for _, want := range []string{"Conectado como Leitora", `action="/pt-BR/signout"`, "Sair"} {
		if !strings.Contains(signedIn, want) {
			t.Errorf("the signed-in header does not carry %s: %q", want, signedIn)
		}
	}
	if strings.Contains(signedIn, "/signin") {
		t.Errorf("the signed-in header offers to sign in: %q", signedIn)
	}

	if nav := accountNav(get(onPlatform(
		httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)))); nav != "" {
		t.Errorf("the platform's page carries an account navigation: %q", nav)
	}
}
