//go:build integration

package httpx_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/aleogr/marketplace/internal/identity"
	"github.com/aleogr/marketplace/internal/identity/breached"
	"github.com/aleogr/marketplace/internal/platform/audit"
	"github.com/aleogr/marketplace/internal/platform/db"
	"github.com/aleogr/marketplace/internal/platform/httpx"
	"github.com/aleogr/marketplace/internal/platform/keys/local"
	"github.com/aleogr/marketplace/internal/platform/ratelimit"
	"github.com/aleogr/marketplace/internal/tenancy"
)

// twoStepSite is the identity routes behind the session middleware, with the
// limits limits makes over the test's database, over a service that seals with
// keys of the test's own, and one confirmed account whose app it returns the
// key of.
func twoStepSite(t *testing.T, limits func(*db.Pool) httpx.IdentityLimits) (http.Handler, *identity.Service, *tenancy.Marketplace, string) {
	t.Helper()
	pool, marketplace := migrated(t)
	keeper, err := local.Generate()
	if err != nil {
		t.Fatal(err)
	}
	service := identity.NewService(serving{t, pool}, identity.NewHasher(cheap, 2), breached.Fake{}, unaudited{}, silent()).
		WithSealer(audit.NewKeys(keeper))
	handler := httpx.Sessions(service, silent())(identityHandler(t, service, limits(pool)))
	confirmedAccount(t, pool, service, marketplace, "Leitora", "leitora@example.test", "correct horse battery staple")
	return handler, service, marketplace, enrolledApp(t, service, marketplace)
}

// enrolledApp adds an app to the fixture's account through the service and
// returns its key.
func enrolledApp(t *testing.T, service *identity.Service, marketplace *tenancy.Marketplace) string {
	t.Helper()
	v := identity.Visit{Marketplace: marketplace.ID, MarketplaceName: marketplace.Name, Language: "pt-BR"}
	token, err := service.SignIn(t.Context(), v, "leitora@example.test", "correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	session, err := service.Authenticate(t.Context(), marketplace.ID, token)
	if err != nil {
		t.Fatal(err)
	}
	enrolment, err := service.BeginApp(t.Context(), v, session)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.ConfirmApp(t.Context(), v, session, "Celular", appCodeFor(t, enrolment.Key, time.Now())); err != nil {
		t.Fatal(err)
	}
	return enrolment.Key
}

// A sign-in through the pages on an account with an app: the password opens
// the second step with a challenge cookie and no session; the app's code opens
// the session, renews the CSRF secret and clears the challenge.
func TestSigningInWithAnAppThroughThePages(t *testing.T) {
	never := ratelimit.Never{}
	handler, _, marketplace, key := twoStepSite(t, func(*db.Pool) httpx.IdentityLimits {
		return httpx.IdentityLimits{
			SignUp: never, Resend: never, ResendAddress: never, SignIn: never, SignInAddress: never, Password: never,
		}
	})
	serve := func(r *http.Request, challenge string) *httptest.ResponseRecorder {
		if challenge != "" {
			r.AddCookie(&http.Cookie{Name: "challenge", Value: challenge})
		}
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, in(r, marketplace, "203.0.113.11"))
		return recorder
	}

	password := serve(formRequest(t, "/signin",
		url.Values{"email": {"leitora@example.test"}, "password": {"correct horse battery staple"}}), "")
	challenge := cookie(password, "challenge")
	if password.Code != http.StatusSeeOther || password.Header().Get("Location") != "/pt-BR/signin/verify" ||
		challenge == nil || challenge.Value == "" || !challenge.HttpOnly {
		t.Fatalf("the password: status %d, Location %q, challenge %+v", password.Code, password.Header().Get("Location"), challenge)
	}
	if cookie(password, "session") != nil {
		t.Fatal("the password alone set a session cookie")
	}

	page := serve(httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/signin/verify", nil), challenge.Value)
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), "Digite o código que seu aplicativo autenticador mostra agora.") ||
		!strings.Contains(page.Body.String(), "Usar um código de recuperação") {
		t.Fatalf("the second step: status %d, body %s", page.Code, page.Body.String())
	}

	wrong := serve(formRequest(t, "/signin/verify", url.Values{"method": {"totp"}, "code": {"000000"}}), challenge.Value)
	if wrong.Code != http.StatusUnauthorized || !strings.Contains(wrong.Body.String(), "Esse código não confere. Tente de novo.") {
		t.Fatalf("a wrong code: status %d, body %s", wrong.Code, wrong.Body.String())
	}

	right := serve(formRequest(t, "/signin/verify",
		url.Values{"method": {"totp"}, "code": {appCodeFor(t, key, time.Now().Add(30*time.Second))}}), challenge.Value)
	if right.Code != http.StatusSeeOther || right.Header().Get("Location") != "/pt-BR/" {
		t.Fatalf("the right code: status %d, Location %q, body %s", right.Code, right.Header().Get("Location"), right.Body.String())
	}
	if session := cookie(right, "session"); session == nil || session.Value == "" {
		t.Fatal("the second step set no session cookie")
	}
	if csrf := cookie(right, "csrf"); csrf == nil || csrf.Value == "" {
		t.Error("the second step did not renew the CSRF secret")
	}
	if cleared := cookie(right, "challenge"); cleared == nil || cleared.MaxAge >= 0 {
		t.Errorf("the challenge cookie was not cleared: %+v", cleared)
	}
	if again := serve(httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/signin/verify", nil), challenge.Value); again.Header().Get("Location") != "/pt-BR/signin?again=expired" {
		t.Fatalf("the spent challenge's page: status %d, Location %q", again.Code, again.Header().Get("Location"))
	}
}

// The F13 per-address limit covers the password and the second step
// together: the attempts of both add up, whichever client makes them, and
// another address is still served.
func TestTheSignInLimitCoversBothSteps(t *testing.T) {
	never := ratelimit.Never{}
	handler, _, marketplace, _ := twoStepSite(t, func(pool *db.Pool) httpx.IdentityLimits {
		return httpx.IdentityLimits{
			SignUp: never, Resend: never, ResendAddress: never, SignIn: never, Password: never,
			SignInAddress: ratelimit.NewDatabase(pool, "signin-address", 4, 15*time.Minute),
		}
	})
	n := 0
	serve := func(r *http.Request, challenge string) *httptest.ResponseRecorder {
		n++
		if challenge != "" {
			r.AddCookie(&http.Cookie{Name: "challenge", Value: challenge})
		}
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, in(r, marketplace, fmt.Sprintf("198.51.100.%d", n)))
		return recorder
	}

	password := serve(formRequest(t, "/signin",
		url.Values{"email": {"leitora@example.test"}, "password": {"correct horse battery staple"}}), "")
	challenge := cookie(password, "challenge")
	if challenge == nil {
		t.Fatalf("the password opened no challenge: status %d", password.Code)
	}
	for attempt := 2; attempt <= 4; attempt++ {
		wrong := serve(formRequest(t, "/signin/verify", url.Values{"method": {"totp"}, "code": {"000000"}}), challenge.Value)
		if wrong.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d = %d, want %d", attempt, wrong.Code, http.StatusUnauthorized)
		}
	}
	if limited := serve(formRequest(t, "/signin/verify", url.Values{"method": {"totp"}, "code": {"000000"}}), challenge.Value); limited.Code != http.StatusTooManyRequests {
		t.Fatalf("the fifth attempt of the address, across both steps = %d, want %d", limited.Code, http.StatusTooManyRequests)
	}
	if limited := serve(formRequest(t, "/signin",
		url.Values{"email": {"leitora@example.test"}, "password": {"correct horse battery staple"}}), ""); limited.Code != http.StatusTooManyRequests {
		t.Fatalf("the password after the limit = %d, want %d", limited.Code, http.StatusTooManyRequests)
	}
	if served := serve(formRequest(t, "/signin",
		url.Values{"email": {"outra@example.test"}, "password": {"not the password at all"}}), ""); served.Code != http.StatusUnauthorized {
		t.Fatalf("another address = %d, want %d: the limit is per address", served.Code, http.StatusUnauthorized)
	}
}
