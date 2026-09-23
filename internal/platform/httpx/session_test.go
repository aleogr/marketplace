package httpx_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/aleogr/marketplace/internal/identity"
	"github.com/aleogr/marketplace/internal/platform/httpx"
	"github.com/aleogr/marketplace/internal/platform/i18n"
	"github.com/aleogr/marketplace/internal/platform/seo"
	"github.com/aleogr/marketplace/internal/tenancy"
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

// overHTTPS is a request as it reaches the service behind the TLS-terminating
// proxy, which is how every deployment is reached.
func overHTTPS(r *http.Request) *http.Request {
	r.Header.Set("X-Forwarded-Proto", "https")
	return r
}

// Over HTTPS the session cookie is `__Host-session`: Secure, HttpOnly, Lax,
// path /, no domain, which is what the browser holds the prefix to. Sign-out
// clears that name, and the plain-HTTP name is not read at all.
func TestOverHTTPSTheSessionCookieIsHostBound(t *testing.T) {
	check := func(t *testing.T, recorder *httptest.ResponseRecorder, cleared bool) {
		t.Helper()
		var found *http.Cookie
		for _, c := range recorder.Result().Cookies() {
			switch c.Name {
			case "__Host-session":
				found = c
			case "session":
				t.Errorf("the plain-HTTP cookie was set over HTTPS: %+v", c)
			}
		}
		if found == nil {
			t.Fatal("no __Host-session cookie was set")
		}
		if !found.Secure || !found.HttpOnly || found.SameSite != http.SameSiteLaxMode ||
			found.Path != "/" || found.Domain != "" {
			t.Errorf("the cookie's flags: %+v", found)
		}
		if cleared != (found.MaxAge < 0) {
			t.Errorf("MaxAge = %d; cleared = %v", found.MaxAge, cleared)
		}
	}

	t.Run("set", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		httpx.SetSession(recorder, overHTTPS(inMarketplace(
			httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/signin", nil))), "token")
		check(t, recorder, false)
	})
	t.Run("cleared at sign-out", func(t *testing.T) {
		// No cookie is sent, so the service, which has no database here, is
		// not asked to revoke anything.
		recorder := httptest.NewRecorder()
		identitySite(t).ServeHTTP(recorder, overHTTPS(inMarketplace(
			httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/signout", nil))))
		check(t, recorder, true)
	})
	t.Run("the plain name is not read", func(t *testing.T) {
		var seen bool
		handler := httpx.Sessions(nil, silent())(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			_, seen = httpx.SessionFrom(r.Context())
		}))
		request := overHTTPS(inMarketplace(httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)))
		request.AddCookie(&http.Cookie{Name: "session", Value: "planted over plain HTTP"})
		handler.ServeHTTP(httptest.NewRecorder(), request)
		if seen {
			t.Fatal("a plain session cookie was read over HTTPS")
		}
	})
}

// A marketplace that is not serving yet has no pages to sign in to, so its
// in-preparation answer offers no way in; nor does a process with no
// identity routes, where the links would lead nowhere.
func TestNoAccountNavigationWhereThereAreNoAccounts(t *testing.T) {
	catalogue, err := i18n.Load()
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	httpx.NewPages(catalogue).InPreparation(recorder,
		httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil),
		&tenancy.Marketplace{ID: "00000000-0000-0000-0000-000000000002", Name: "Loja Dois",
			State: tenancy.InPreparation, DefaultLanguage: "pt-BR", Languages: []string{"pt-BR"}})
	if !strings.Contains(recorder.Body.String(), "Loja Dois") {
		t.Fatalf("the in-preparation page was not rendered: %s", recorder.Body.String())
	}
	if nav := accountNav(recorder.Body.String()); nav != "" {
		t.Errorf("the in-preparation page carries an account navigation: %q", nav)
	}

	preview, err := seo.NewPreview()
	if err != nil {
		t.Fatal(err)
	}
	recorder = httptest.NewRecorder()
	httpx.NewSite(nil, catalogue, preview, false).Handler().ServeHTTP(recorder,
		inMarketplace(httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)))
	if nav := accountNav(recorder.Body.String()); nav != "" {
		t.Errorf("a site with no identity routes carries an account navigation: %q", nav)
	}
}

// The User-Agent is the visitor's to write, up to the server's header limit,
// and it is stored with every session and every refused sign-in: it is cut
// to a bound, whole characters only, and made valid UTF-8, which is all a
// text column accepts.
func TestTheUserAgentIsBoundedAndValid(t *testing.T) {
	for name, tc := range map[string]struct{ sent, want string }{
		"short":   {"Mozilla/5.0 (X11)", "Mozilla/5.0 (X11)"},
		"long":    {strings.Repeat("é", 400), strings.Repeat("é", 256)},
		"odd cut": {"x" + strings.Repeat("é", 400), "x" + strings.Repeat("é", 255)},
		"invalid": {"agent \xff\xfe", "agent \uFFFD"},
	} {
		t.Run(name, func(t *testing.T) {
			request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/signin", nil)
			request.Header.Set("User-Agent", tc.sent)
			got := httpx.VisitOf(inMarketplace(request)).UserAgent
			if got != tc.want {
				t.Errorf("UserAgent = %q, want %q", got, tc.want)
			}
			if len(got) > 512 || !utf8.ValidString(got) {
				t.Errorf("UserAgent is %d bytes, valid UTF-8 %v", len(got), utf8.ValidString(got))
			}
		})
	}
}
