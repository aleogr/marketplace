package httpx_test

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/aleogr/marketplace/internal/identity"
	"github.com/aleogr/marketplace/internal/identity/breached"
	"github.com/aleogr/marketplace/internal/platform/httpx"
	"github.com/aleogr/marketplace/internal/platform/i18n"
	"github.com/aleogr/marketplace/internal/platform/ratelimit"
	"github.com/aleogr/marketplace/internal/platform/seo"
	"github.com/aleogr/marketplace/internal/tenancy"
)

// silent is a logger for tests that do not read the log.
func silent() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// cheap keeps the hashing in these tests fast.
var cheap = identity.Params{Memory: 64, Time: 1, Threads: 1, KeyLen: 32, SaltLen: 16}

// identityHandler returns the routes with the identity pages mounted over
// service, limited by limits.
func identityHandler(t *testing.T, service *identity.Service, limits httpx.IdentityLimits) http.Handler {
	t.Helper()

	catalogue, err := i18n.Load()
	if err != nil {
		t.Fatalf("i18n.Load() = %v", err)
	}
	preview, err := seo.NewPreview()
	if err != nil {
		t.Fatalf("seo.NewPreview() = %v", err)
	}
	return httpx.NewSite(nil, catalogue, preview, false).WithIdentity(httpx.IdentityRoutes{
		Service: service,
		Limits:  limits,
		Pages:   httpx.NewPages(catalogue),
		Log:     silent(),
	}).Handler()
}

// identitySite is identityHandler over a service with no database, and no
// limits. The tests that use it stop before the database: what they prove is
// the part of each page that needs none.
func identitySite(t *testing.T) http.Handler {
	t.Helper()
	never := ratelimit.Never{}
	return identityHandler(t,
		identity.NewService(nil, identity.NewHasher(cheap, 1), breached.Fake{}, nil, silent()),
		httpx.IdentityLimits{
			SignUp: never, Resend: never, ResendAddress: never,
			SignIn: never, SignInAddress: never, Password: never,
		})
}

// inMarketplace is a request as it reaches the routes: its host resolved to a
// marketplace and its language to pt-BR.
func inMarketplace(r *http.Request) *http.Request {
	ctx := tenancy.WithResolution(r.Context(), tenancy.Resolution{
		Kind: tenancy.MarketplaceHost,
		Marketplace: &tenancy.Marketplace{
			ID: "00000000-0000-0000-0000-000000000001", Name: "Loja Um",
			State: tenancy.Active, DefaultLanguage: "pt-BR", Languages: []string{"pt-BR", "en-US"},
		},
	})
	return r.WithContext(i18n.WithLanguage(ctx, "pt-BR"))
}

// formRequest is a form posted to path in the marketplace.
func formRequest(t *testing.T, path string, form url.Values) *http.Request {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, path, strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return inMarketplace(request)
}

// postForm sends a form to the identity routes and returns the response.
func postForm(t *testing.T, path string, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	identitySite(t).ServeHTTP(recorder, formRequest(t, path, form))
	return recorder
}

// The validation that needs no database answers first, in the visitor's
// language, as an HTML page with the status that says the form was refused.
func TestSignUpShowsTheFieldErrorBeforeTouchingTheDatabase(t *testing.T) {
	recorder := postForm(t, "/signup",
		url.Values{"name": {""}, "email": {"a@example.test"}, "password": {"correct horse battery"}})

	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnprocessableEntity)
	}
	if got := recorder.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Errorf("Content-Type = %q, want the page's", got)
	}
	if body := recorder.Body.String(); !strings.Contains(body, "Informe seu nome.") {
		t.Errorf("the page does not say what is missing: %s", body)
	}
}

// The minimum a visitor reads is the minimum the rule applies: both come from
// identity.MinPasswordLength.
func TestTheShortPasswordMessageNamesTheRulesMinimum(t *testing.T) {
	recorder := postForm(t, "/signup",
		url.Values{"name": {"Leitora"}, "email": {"a@example.test"}, "password": {"curta"}})

	want := "A senha precisa de pelo menos 12 caracteres."
	if recorder.Code != http.StatusUnprocessableEntity || !strings.Contains(recorder.Body.String(), want) {
		t.Fatalf("status %d, body without %q: %s", recorder.Code, want, recorder.Body.String())
	}
}

func TestTheSignUpFieldCarriesTheRulesBounds(t *testing.T) {
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/signup", nil)
	recorder := httptest.NewRecorder()
	identitySite(t).ServeHTTP(recorder, inMarketplace(request))

	body := recorder.Body.String()
	for _, want := range []string{`minlength="12"`, `maxlength="128"`, "No mínimo 12 caracteres."} {
		if !strings.Contains(body, want) {
			t.Errorf("the sign-up page does not carry %s", want)
		}
	}
}

// onPlatform is a request as it reaches the routes on the platform's own
// host, which belongs to no marketplace and so has no accounts.
func onPlatform(r *http.Request) *http.Request {
	ctx := tenancy.WithResolution(r.Context(), tenancy.Resolution{Kind: tenancy.PlatformHost})
	return r.WithContext(i18n.WithLanguage(ctx, "pt-BR"))
}

// The platform's own host has no accounts: its identity routes are not
// found, and a form posted there reaches neither the hasher nor the database
// (identitySite has none, so reaching it would panic).
func TestTheIdentityRoutesAreNotFoundOnThePlatformHost(t *testing.T) {
	form := url.Values{"name": {"Leitora"}, "email": {"a@example.test"}, "password": {"correct horse battery"}}
	for _, route := range []struct{ method, path string }{
		{http.MethodGet, "/signup"}, {http.MethodPost, "/signup"},
		{http.MethodGet, "/verify"}, {http.MethodPost, "/verify"},
		{http.MethodGet, "/verify/resend"}, {http.MethodPost, "/verify/resend"},
	} {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			request := httptest.NewRequestWithContext(t.Context(), route.method, route.path,
				strings.NewReader(form.Encode()))
			request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			recorder := httptest.NewRecorder()
			identitySite(t).ServeHTTP(recorder, onPlatform(request))

			if recorder.Code != http.StatusNotFound {
				t.Errorf("status = %d, want %d", recorder.Code, http.StatusNotFound)
			}
		})
	}
}

// The address a mailed link points at is the host the request resolved by,
// normalised, with a port only when the port is a number: the rest of the Host
// header is the sender's to write, and the link goes to somebody else.
func TestTheMailedLinkBaseIsTheResolvedHost(t *testing.T) {
	for _, tc := range []struct{ host, proto, want string }{
		{"M1.Example.com.", "", "http://m1.example.com"},
		{"m1.example.com", "https", "https://m1.example.com"},
		{"m1.localhost:8080", "", "http://m1.localhost:8080"},
		{"m1.example.com:evil.example", "", "http://m1.example.com"},
		{"m1.example.com:99999", "", "http://m1.example.com"},
		{"m1.example.com:+80", "", "http://m1.example.com"},
		{"m1.example.com:", "", "http://m1.example.com"},
	} {
		t.Run(tc.host, func(t *testing.T) {
			request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/signup", nil)
			request.Host = tc.host
			if tc.proto != "" {
				request.Header.Set("X-Forwarded-Proto", tc.proto)
			}
			if got := httpx.LinkBase(request); got != tc.want {
				t.Errorf("LinkBase(%q) = %q, want %q", tc.host, got, tc.want)
			}
		})
	}
}
