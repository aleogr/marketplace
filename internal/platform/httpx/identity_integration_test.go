//go:build integration

package httpx_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/aleogr/marketplace/internal/identity"
	"github.com/aleogr/marketplace/internal/identity/breached"
	"github.com/aleogr/marketplace/internal/platform/audit"
	"github.com/aleogr/marketplace/internal/platform/config"
	"github.com/aleogr/marketplace/internal/platform/db"
	"github.com/aleogr/marketplace/internal/platform/dbtest"
	"github.com/aleogr/marketplace/internal/platform/httpx"
	"github.com/aleogr/marketplace/internal/platform/i18n"
	"github.com/aleogr/marketplace/internal/platform/ratelimit"
	"github.com/aleogr/marketplace/internal/tenancy"
)

func TestMain(m *testing.M) { os.Exit(dbtest.Run(m)) }

// serving is a Transactor that runs as the application role, the way the
// deployed service does, so row-level security is what is tested.
type serving struct {
	t    *testing.T
	pool *db.Pool
}

func (s serving) InTxFor(_ context.Context, marketplaceID string, fn func(pgx.Tx) error) error {
	return dbtest.Serving(s.t, s.pool, marketplaceID, fn)
}

// unaudited is an auditor that keeps nothing: what these tests prove is the
// pages, and the audit trail of each flow is internal/identity's to test.
type unaudited struct{}

func (unaudited) Append(context.Context, pgx.Tx, audit.Entry) error { return nil }

// unique returns a name no other test has used. The tests of this package
// share one database (internal/platform/dbtest).
func unique(t *testing.T, name string) string {
	t.Helper()
	raw := make([]byte, 6)
	if _, err := rand.Read(raw); err != nil {
		t.Fatal(err)
	}
	return name + "-" + hex.EncodeToString(raw)
}

// migrated opens the package's migrated database, with the application role
// ready, and seeds a marketplace of this test's own.
func migrated(t *testing.T) (*db.Pool, *tenancy.Marketplace) {
	t.Helper()
	pool, err := db.Open(t.Context(), config.Database{URL: dbtest.URL(t)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	dbtest.AsApplication(t, pool)

	slug := unique(t, "loja")
	spec := tenancy.Spec{Slug: slug, Name: "Loja", Market: "BR", RevenueModel: "commission", State: "active",
		DefaultLanguage: "pt-BR", Languages: []string{"pt-BR"}, Hosts: []string{slug + ".test"}}
	var applied []tenancy.Applied
	if err := pool.InTx(t.Context(), func(tx pgx.Tx) error {
		applied, err = tenancy.Seed(t.Context(), tx, []tenancy.Spec{spec})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return pool, &tenancy.Marketplace{ID: applied[0].ID, Name: spec.Name, State: tenancy.Active,
		DefaultLanguage: "pt-BR", Languages: []string{"pt-BR"}}
}

// in places a request in marketplace, in pt-BR, from ip.
func in(r *http.Request, marketplace *tenancy.Marketplace, ip string) *http.Request {
	ctx := tenancy.WithResolution(r.Context(), tenancy.Resolution{Kind: tenancy.MarketplaceHost, Marketplace: marketplace})
	ctx = httpx.WithOrigin(i18n.WithLanguage(ctx, "pt-BR"), httpx.Origin{IP: ip})
	return r.WithContext(ctx)
}

// A credential-stuffing run tries many passwords against one address, from
// many addresses of its own. The per-address limit is what stops it: the
// eleventh attempt within the window is refused whichever client sends it,
// while another address is still served (docs/roadmap.md, F13).
func TestCredentialStuffingAgainstOneAddressIsStopped(t *testing.T) {
	pool, marketplace := migrated(t)
	service := identity.NewService(serving{t, pool}, identity.NewHasher(cheap, 2), breached.Fake{}, unaudited{}, silent())
	handler := identityHandler(t, service, httpx.IdentityLimits{
		SignIn:        ratelimit.NewDatabase(pool, "signin", 1000, 10*time.Minute),
		SignInAddress: ratelimit.NewDatabase(pool, "signin-address", 10, 15*time.Minute),
	})

	attempt := func(email string, n int) int {
		request := formRequest(t, "/signin",
			url.Values{"email": {email}, "password": {fmt.Sprintf("guess number %04d", n)}})
		// A different client each time, as a botnet would be.
		request = in(request, marketplace, fmt.Sprintf("198.51.100.%d", n+1))
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		return recorder.Code
	}

	for n := range 10 {
		if code := attempt("alvo@example.test", n); code != http.StatusUnauthorized {
			t.Fatalf("attempt %d = %d, want %d: refused as a wrong password", n+1, code, http.StatusUnauthorized)
		}
	}
	if code := attempt("alvo@example.test", 10); code != http.StatusTooManyRequests {
		t.Fatalf("the eleventh attempt = %d, want %d", code, http.StatusTooManyRequests)
	}
	if code := attempt("outra@example.test", 11); code != http.StatusUnauthorized {
		t.Fatalf("another address = %d, want %d: the limit is per address", code, http.StatusUnauthorized)
	}
}

// cookie returns the cookie a response sets under name, or nil.
func cookie(recorder *httptest.ResponseRecorder, name string) *http.Cookie {
	var found *http.Cookie
	for _, c := range recorder.Result().Cookies() {
		if c.Name == name {
			found = c
		}
	}
	return found
}

// Sign-in through the real handler and database: the refusals, the session
// cookie and its flags, the account in the page of the next request, and
// sign-out ending the session so the same cookie no longer opens it.
func TestSignInReadTheSessionAndSignOut(t *testing.T) {
	pool, marketplace := migrated(t)
	service := identity.NewService(serving{t, pool}, identity.NewHasher(cheap, 2), breached.Fake{}, unaudited{}, silent())
	never := ratelimit.Never{}
	routes := identityHandler(t, service, httpx.IdentityLimits{
		SignUp: never, Resend: never, ResendAddress: never, SignIn: never, SignInAddress: never, Password: never,
	})
	handler := httpx.Sessions(service, silent())(routes)
	serve := func(r *http.Request) *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, in(r, marketplace, "203.0.113.7"))
		return recorder
	}
	signIn := func(email, password string) *httptest.ResponseRecorder {
		return serve(formRequest(t, "/signin", url.Values{"email": {email}, "password": {password}}))
	}

	const password = "correct horse battery staple"
	visit := identity.Visit{Marketplace: marketplace.ID, MarketplaceName: marketplace.Name, Language: "pt-BR",
		BaseURL: "http://loja.test"}
	if err := service.SignUp(t.Context(), visit, "Leitora", "leitora@example.test", password); err != nil {
		t.Fatal(err)
	}

	// Unconfirmed: the right password is told to confirm the address first.
	if recorder := signIn("leitora@example.test", password); recorder.Code != http.StatusUnauthorized ||
		!strings.Contains(recorder.Body.String(), "Confirme seu e-mail primeiro.") {
		t.Fatalf("unconfirmed: status %d, body %s", recorder.Code, recorder.Body.String())
	}

	// The confirmation itself is internal/identity's to test; here the
	// account is simply confirmed.
	if _, err := pool.Exec(t.Context(),
		"UPDATE account SET verified_at = now() WHERE marketplace_id = $1 AND email_normalised = $2",
		marketplace.ID, "leitora@example.test"); err != nil {
		t.Fatal(err)
	}

	// A wrong password and an unknown address read the same (D7).
	wrong, unknown := signIn("leitora@example.test", "not the password at all"), signIn("ninguem@example.test", password)
	for _, recorder := range []*httptest.ResponseRecorder{wrong, unknown} {
		if recorder.Code != http.StatusUnauthorized ||
			!strings.Contains(recorder.Body.String(), "O e-mail ou a senha não conferem.") {
			t.Fatalf("refused sign-in: status %d, body %s", recorder.Code, recorder.Body.String())
		}
		if cookie(recorder, "session") != nil {
			t.Fatal("a refused sign-in set a session cookie")
		}
	}

	signedIn := signIn("Leitora@Example.test", password)
	if signedIn.Code != http.StatusSeeOther || signedIn.Header().Get("Location") != "/pt-BR/" {
		t.Fatalf("sign-in: status %d, Location %q", signedIn.Code, signedIn.Header().Get("Location"))
	}
	session := cookie(signedIn, "session")
	if session == nil || session.Value == "" {
		t.Fatal("sign-in set no session cookie")
	}
	if !session.HttpOnly || session.SameSite != http.SameSiteLaxMode || session.Path != "/" || session.Domain != "" {
		t.Errorf("the session cookie's flags: %+v", session)
	}
	if csrf := cookie(signedIn, "csrf"); csrf == nil || csrf.Value == "" {
		t.Error("sign-in did not renew the CSRF secret")
	}

	home := func() *httptest.ResponseRecorder {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
		request.AddCookie(&http.Cookie{Name: "session", Value: session.Value})
		return serve(request)
	}
	signedInHome := home()
	if body := signedInHome.Body.String(); !strings.Contains(body, "Conectado como Leitora") {
		t.Fatalf("the next page does not know who is signed in: %s", body)
	}
	// A signed-in page names the account, so no cache may keep it: not a
	// proxy, and not the browser's back button after sign-out.
	if got := signedInHome.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("a signed-in page's Cache-Control = %q, want no-store", got)
	}

	out := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/signout", nil)
	out.AddCookie(&http.Cookie{Name: "session", Value: session.Value})
	signedOut := serve(out)
	if signedOut.Code != http.StatusSeeOther {
		t.Fatalf("sign-out: status %d", signedOut.Code)
	}
	if cleared := cookie(signedOut, "session"); cleared == nil || cleared.MaxAge >= 0 {
		t.Errorf("sign-out did not clear the cookie: %+v", cleared)
	}

	// The same cookie, sent again, opens nothing, and is cleared.
	after := home()
	if strings.Contains(after.Body.String(), "Conectado como") {
		t.Fatal("a revoked session still signs the page in")
	}
	if cleared := cookie(after, "session"); cleared == nil || cleared.MaxAge >= 0 {
		t.Errorf("the revoked session's cookie was not cleared: %+v", cleared)
	}
}

// confirmedAccount signs an account up through the service and confirms it.
// The confirmation itself is internal/identity's to test.
func confirmedAccount(t *testing.T, pool *db.Pool, service *identity.Service, marketplace *tenancy.Marketplace,
	name, email, password string) {
	t.Helper()
	visit := identity.Visit{Marketplace: marketplace.ID, MarketplaceName: marketplace.Name, Language: "pt-BR",
		BaseURL: "http://loja.test"}
	if err := service.SignUp(t.Context(), visit, name, email, password); err != nil {
		t.Fatal(err)
	}
	normalised, err := identity.NormaliseEmail(email)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(t.Context(),
		"UPDATE account SET verified_at = now() WHERE marketplace_id = $1 AND email_normalised = $2",
		marketplace.ID, normalised); err != nil {
		t.Fatal(err)
	}
}

// Signing in again in a browser that holds a session, even as another
// account, replaces the cookie; the session it held is revoked on the server,
// not left valid for thirty idle days with nobody holding its cookie.
func TestSigningInAgainRevokesTheSessionItReplaces(t *testing.T) {
	pool, marketplace := migrated(t)
	service := identity.NewService(serving{t, pool}, identity.NewHasher(cheap, 2), breached.Fake{}, unaudited{}, silent())
	never := ratelimit.Never{}
	handler := httpx.Sessions(service, silent())(identityHandler(t, service, httpx.IdentityLimits{
		SignUp: never, Resend: never, ResendAddress: never, SignIn: never, SignInAddress: never, Password: never,
	}))
	const password = "correct horse battery staple"
	confirmedAccount(t, pool, service, marketplace, "Primeira", "primeira@example.test", password)
	confirmedAccount(t, pool, service, marketplace, "Segunda", "segunda@example.test", password)

	signIn := func(email string, held *http.Cookie) string {
		t.Helper()
		request := formRequest(t, "/signin", url.Values{"email": {email}, "password": {password}})
		if held != nil {
			request.AddCookie(held)
		}
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, in(request, marketplace, "203.0.113.8"))
		got := cookie(recorder, "session")
		if recorder.Code != http.StatusSeeOther || got == nil {
			t.Fatalf("sign-in as %s: status %d, cookie %+v", email, recorder.Code, got)
		}
		return got.Value
	}

	first := signIn("primeira@example.test", nil)
	second := signIn("segunda@example.test", &http.Cookie{Name: "session", Value: first})

	if _, err := service.Authenticate(t.Context(), marketplace.ID, first); !errors.Is(err, identity.ErrSessionInvalid) {
		t.Errorf("the replaced session still authenticates: %v", err)
	}
	if _, err := service.Authenticate(t.Context(), marketplace.ID, second); err != nil {
		t.Errorf("the new session does not authenticate: %v", err)
	}
}

// A password change through the real handler and database: a wrong current
// password is refused with its own sentence, the right one changes it, the
// browser that changed it stays signed in on a new session cookie with a new
// CSRF secret, is redirected to the page that says so, and the other browser
// and the old cookie are signed out.
// The limit counts per account: once one account has used its attempts,
// another account is still served.
func TestChangingThePasswordThroughThePage(t *testing.T) {
	pool, marketplace := migrated(t)
	service := identity.NewService(serving{t, pool}, identity.NewHasher(cheap, 2), breached.Fake{}, unaudited{}, silent())
	never := ratelimit.Never{}
	handler := httpx.Sessions(service, silent())(identityHandler(t, service, httpx.IdentityLimits{
		SignUp: never, Resend: never, ResendAddress: never, SignIn: never, SignInAddress: never,
		Password: ratelimit.NewDatabase(pool, "password", 2, time.Hour),
	}))
	const password = "correct horse battery staple"
	confirmedAccount(t, pool, service, marketplace, "Leitora", "leitora@example.test", password)
	confirmedAccount(t, pool, service, marketplace, "Outra", "outra@example.test", password)
	visit := identity.Visit{Marketplace: marketplace.ID, Language: "pt-BR"}
	open := func(email string) string {
		t.Helper()
		token, err := service.SignIn(t.Context(), visit, email, password)
		if err != nil {
			t.Fatal(err)
		}
		return token
	}
	here, there, other := open("leitora@example.test"), open("leitora@example.test"), open("outra@example.test")

	change := func(token, current, next string) *httptest.ResponseRecorder {
		t.Helper()
		request := formRequest(t, "/account/password",
			url.Values{"current_password": {current}, "new_password": {next}})
		request.AddCookie(&http.Cookie{Name: "session", Value: token})
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, in(request, marketplace, "203.0.113.9"))
		return recorder
	}

	wrong := change(here, "not the password at all", "a brand new passphrase")
	if wrong.Code != http.StatusUnprocessableEntity || !strings.Contains(wrong.Body.String(), "A senha atual não confere.") {
		t.Fatalf("wrong current password: status %d, body %s", wrong.Code, wrong.Body.String())
	}
	done := change(here, password, "a brand new passphrase")
	if done.Code != http.StatusSeeOther || done.Header().Get("Location") != "/pt-BR/account/password?changed=1" {
		t.Fatalf("password change: status %d, Location %q", done.Code, done.Header().Get("Location"))
	}
	renewed := cookie(done, "session")
	if renewed == nil || renewed.Value == "" || renewed.Value == here {
		t.Fatalf("the password change set no new session cookie: %+v", renewed)
	}
	if csrf := cookie(done, "csrf"); csrf == nil || csrf.Value == "" {
		t.Error("the password change did not renew the CSRF secret")
	}
	if _, err := service.Authenticate(t.Context(), marketplace.ID, renewed.Value); err != nil {
		t.Errorf("the browser that changed the password was signed out: %v", err)
	}
	for name, token := range map[string]string{"the old cookie": here, "the other browser": there} {
		if _, err := service.Authenticate(t.Context(), marketplace.ID, token); !errors.Is(err, identity.ErrSessionInvalid) {
			t.Errorf("%s is still signed in: %v", name, err)
		}
	}

	shown := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/account/password?changed=1", nil)
	shown.AddCookie(&http.Cookie{Name: "session", Value: renewed.Value})
	page := httptest.NewRecorder()
	handler.ServeHTTP(page, in(shown, marketplace, "203.0.113.9"))
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), "Sua senha foi alterada.") {
		t.Fatalf("the page after the change: status %d, body %s", page.Code, page.Body.String())
	}
	if got := page.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("the signed-in page's Cache-Control = %q, want no-store", got)
	}

	if limited := change(renewed.Value, "a brand new passphrase", "yet another passphrase"); limited.Code != http.StatusTooManyRequests {
		t.Errorf("the third attempt of one account = %d, want %d", limited.Code, http.StatusTooManyRequests)
	}
	if served := change(other, "not the password at all", "a brand new passphrase"); served.Code != http.StatusUnprocessableEntity {
		t.Errorf("another account = %d, want %d: the limit is per account", served.Code, http.StatusUnprocessableEntity)
	}
}
