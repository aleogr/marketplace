//go:build integration

package httpx_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
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
	"github.com/aleogr/marketplace/internal/platform/keys/local"
	"github.com/aleogr/marketplace/internal/platform/ratelimit"
	"github.com/aleogr/marketplace/internal/platform/version"
	"github.com/aleogr/marketplace/internal/staff"
	"github.com/aleogr/marketplace/internal/tenancy"
)

const (
	ownerEmail    = "owner@example.test"
	ownerPassword = "correct horse battery staple"
	bootToken     = "a-bootstrap-token-of-forty-eight-characters-0123"
)

// console is the site with the console mounted, over a database of the
// test's own — the owner is created once per database — holding one
// marketplace and the owner, created by the first run.
type console struct {
	pool        *db.Pool
	service     *identity.Service
	handler     http.Handler
	marketplace *tenancy.Marketplace
	bootstrap   *staff.Bootstrap
}

func newConsole(t *testing.T, withOwner bool) console {
	t.Helper()
	pool, err := db.Open(t.Context(), config.Database{URL: dbtest.Fresh(t)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	dbtest.AsApplication(t, pool)

	spec := tenancy.Spec{Slug: "loja", Name: "Loja", Market: "BR", RevenueModel: "commission", State: "active",
		DefaultLanguage: "pt-BR", Languages: []string{"pt-BR"}, Hosts: []string{"loja.test"}}
	var applied []tenancy.Applied
	if err := pool.InTx(t.Context(), func(tx pgx.Tx) error {
		applied, err = tenancy.Seed(t.Context(), tx, []tenancy.Spec{spec})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	marketplace := &tenancy.Marketplace{ID: applied[0].ID, Name: spec.Name, State: tenancy.Active,
		DefaultLanguage: "pt-BR", Languages: []string{"pt-BR"}}

	keeper, err := local.Generate()
	if err != nil {
		t.Fatal(err)
	}
	database := serving{t, pool}
	service := identity.NewService(database, identity.NewHasher(cheap, 2), breached.Fake{}, unaudited{}, silent()).
		WithSealer(audit.NewKeys(keeper))
	bootstrap := staff.NewBootstrap(database, service, unaudited{}, bootToken)
	if withOwner {
		if _, err := bootstrap.Setup(t.Context(), identity.Visit{Platform: true, Language: "pt-BR"},
			bootToken, "Dona", ownerEmail, ownerPassword); err != nil {
			t.Fatal(err)
		}
	}

	catalogue, err := i18n.Load()
	if err != nil {
		t.Fatal(err)
	}
	never := ratelimit.Never{}
	site := httpx.NewSite(nil, catalogue, nil, false).WithIdentity(httpx.IdentityRoutes{
		Service: service, Pages: httpx.NewPages(catalogue), Log: silent(),
		Limits: httpx.IdentityLimits{SignUp: never, Resend: never, ResendAddress: never, SignIn: never,
			SignInAddress: never, Password: never, StepUp: never},
	}).WithConsole(httpx.ConsoleRoutes{Authoriser: staff.NewOwners(database)})
	return console{pool: pool, service: service, marketplace: marketplace, bootstrap: bootstrap,
		handler: httpx.Sessions(service, silent())(site.Handler())}
}

// visitor sends requests to a host — the console's, or the marketplace's —
// and keeps the cookies the responses set, as a browser does.
type visitor struct {
	t       *testing.T
	handler http.Handler
	place   func(*http.Request) *http.Request
	cookies map[string]string
}

func (c console) onConsole(t *testing.T) *visitor {
	return &visitor{t: t, handler: c.handler, cookies: map[string]string{}, place: func(r *http.Request) *http.Request {
		ctx := tenancy.WithResolution(r.Context(), tenancy.Resolution{Kind: tenancy.ConsoleHost})
		ctx = httpx.WithOrigin(i18n.WithLanguage(ctx, "pt-BR"), httpx.Origin{IP: "203.0.113.20"})
		return r.WithContext(ctx)
	}}
}

func (c console) onStore(t *testing.T) *visitor {
	return &visitor{t: t, handler: c.handler, cookies: map[string]string{}, place: func(r *http.Request) *http.Request {
		return in(r, c.marketplace, "203.0.113.21")
	}}
}

func (v *visitor) get(path string) *httptest.ResponseRecorder {
	return v.send(httptest.NewRequestWithContext(v.t.Context(), http.MethodGet, path, nil))
}

func (v *visitor) post(path string, form url.Values) *httptest.ResponseRecorder {
	request := httptest.NewRequestWithContext(v.t.Context(), http.MethodPost, path, strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return v.send(request)
}

func (v *visitor) send(request *http.Request) *httptest.ResponseRecorder {
	for name, value := range v.cookies {
		request.AddCookie(&http.Cookie{Name: name, Value: value})
	}
	recorder := httptest.NewRecorder()
	v.handler.ServeHTTP(recorder, v.place(request))
	for _, set := range recorder.Result().Cookies() {
		if set.MaxAge < 0 {
			delete(v.cookies, set.Name)
			continue
		}
		v.cookies[set.Name] = set.Value
	}
	return recorder
}

// signIn posts the sign-in form.
func (v *visitor) signIn(email, password string) *httptest.ResponseRecorder {
	return v.post("/signin", url.Values{"email": {email}, "password": {password}})
}

// wantRedirect fails the test unless recorder is a 303 to location.
func wantRedirect(t *testing.T, recorder *httptest.ResponseRecorder, location string) {
	t.Helper()
	if recorder.Code != http.StatusSeeOther || recorder.Header().Get("Location") != location {
		t.Fatalf("status %d to %q, want 303 to %q; body %s", recorder.Code,
			recorder.Header().Get("Location"), location, recorder.Body.String())
	}
}

// enrolAppOnConsole adds an app from the console's enrolment pages and
// returns its key, having checked the recovery codes were shown.
func enrolAppOnConsole(t *testing.T, v *visitor) string {
	t.Helper()
	page := v.get("/account/security/app")
	found := keyOnPage.FindStringSubmatch(page.Body.String())
	if page.Code != http.StatusOK || found == nil {
		t.Fatalf("the app page: status %d, body %s", page.Code, page.Body.String())
	}
	added := v.post("/account/security/app", url.Values{"label": {"Celular"}, "code": {appCodeFor(t, found[1], time.Now())}})
	if added.Code != http.StatusOK || strings.Count(added.Body.String(), "<li><code>") != identity.RecoveryCodeCount {
		t.Fatalf("adding the app: status %d, body %s", added.Code, added.Body.String())
	}
	return found[1]
}

// The owner signs in on the console with the password, is sent to add an
// app or a key before anything else, sees the recovery codes, and only then
// reaches the console's home, whose shell names them and shows the build;
// signing in again asks for the app.
func TestTheOwnerSignsInAndIsMadeToEnrolFirst(t *testing.T) {
	c := newConsole(t, true)
	v := c.onConsole(t)

	form := v.get("/signin")
	if form.Code != http.StatusOK || strings.Contains(form.Body.String(), "/signup") {
		t.Fatalf("the console's sign-in page: status %d, and it offers a sign-up: %s", form.Code, form.Body.String())
	}
	if refused := v.signIn(ownerEmail, "not the password at all"); refused.Code != http.StatusUnauthorized {
		t.Fatalf("a wrong password = %d, want 401", refused.Code)
	}
	wantRedirect(t, v.signIn(ownerEmail, ownerPassword), "/pt-BR/")

	// No factor yet: every console page sends the owner to enrol one.
	for _, path := range []string{"/", "/account/security", "/account/password"} {
		wantRedirect(t, v.get(path), "/pt-BR/enrol")
	}
	if version := v.get("/console/version"); version.Code != http.StatusNotFound {
		t.Fatalf("the version with no factor = %d, want 404", version.Code)
	}
	enrol := v.get("/enrol")
	if enrol.Code != http.StatusOK || !strings.Contains(enrol.Body.String(), `href="/pt-BR/account/security/key"`) ||
		!strings.Contains(enrol.Body.String(), `href="/pt-BR/account/security/app"`) ||
		strings.Contains(enrol.Body.String(), "/account/security/email") {
		t.Fatalf("the enrolment page: status %d, body %s", enrol.Code, enrol.Body.String())
	}
	key := enrolAppOnConsole(t, v)

	home := v.get("/")
	body := home.Body.String()
	if home.Code != http.StatusOK || !strings.Contains(body, "Dona") || !strings.Contains(body, version.String()) ||
		!strings.Contains(body, `class="menu"`) || !strings.Contains(body, `action="/pt-BR/signout"`) {
		t.Fatalf("the console's home: status %d, body %s", home.Code, body)
	}
	if enrolled := v.get("/enrol"); enrolled.Code != http.StatusSeeOther {
		t.Fatalf("the enrolment page with a factor = %d, want a redirect home", enrolled.Code)
	}
	answer := v.get("/console/version")
	var got map[string]string
	if answer.Code != http.StatusOK || answer.Header().Get("Content-Type") != "application/json" ||
		json.Unmarshal(answer.Body.Bytes(), &got) != nil || got["version"] != version.String() {
		t.Fatalf("the version: status %d, %q, body %s", answer.Code, answer.Header().Get("Content-Type"), answer.Body.String())
	}

	// Signed out and in again: the password opens the second step, and the
	// app's code the console.
	wantRedirect(t, v.post("/signout", nil), "/pt-BR/")
	wantRedirect(t, v.signIn(ownerEmail, ownerPassword), "/pt-BR/signin/verify")
	step := v.get("/signin/verify")
	if step.Code != http.StatusOK || strings.Contains(step.Body.String(), "/signin/verify/email") {
		t.Fatalf("the second step: status %d, body %s", step.Code, step.Body.String())
	}
	wantRedirect(t, v.post("/signin/verify", url.Values{"method": {"totp"},
		"code": {appCodeFor(t, key, time.Now().Add(30*time.Second))}}), "/pt-BR/")
	if again := v.get("/"); again.Code != http.StatusOK {
		t.Fatalf("the home after the second step = %d", again.Code)
	}
}

// The version and the home are "not found" to anybody the authoriser does
// not allow: signed out, and a staff member who is not the owner (D8).
func TestTheConsoleIsNotFoundToWhoeverIsNotAllowed(t *testing.T) {
	c := newConsole(t, true)
	if answer := c.onConsole(t).get("/console/version"); answer.Code != http.StatusNotFound {
		t.Fatalf("the version signed out = %d, want 404", answer.Code)
	}
	wantRedirect(t, c.onConsole(t).get("/"), "/pt-BR/signin")

	if _, err := c.service.CreateStaff(t.Context(), identity.Visit{Platform: true, Language: "pt-BR"},
		"Colega", "colega@example.test", ownerPassword, nil); err != nil {
		t.Fatal(err)
	}
	colleague := c.onConsole(t)
	wantRedirect(t, colleague.signIn("colega@example.test", ownerPassword), "/pt-BR/")
	enrolAppOnConsole(t, colleague)
	for _, path := range []string{"/", "/console/version"} {
		if answer := colleague.get(path); answer.Code != http.StatusNotFound {
			t.Errorf("%s for a staff member who is not the owner = %d, want 404", path, answer.Code)
		}
	}
	// Their own security page is theirs, whatever they may do elsewhere.
	if security := colleague.get("/account/security"); security.Code != http.StatusOK {
		t.Errorf("the colleague's own security page = %d, want 200", security.Code)
	}
}

// Buyers never sign in on the console's host, and staff never on a store's:
// each host sees only its own accounts, and a cookie of one opens nothing on
// the other.
func TestBuyersAndStaffEachSignInOnlyOnTheirOwnHost(t *testing.T) {
	c := newConsole(t, true)
	confirmedAccount(t, c.pool, c.service, c.marketplace, "Leitora", "leitora@example.test", ownerPassword)

	if refused := c.onConsole(t).signIn("leitora@example.test", ownerPassword); refused.Code != http.StatusUnauthorized {
		t.Fatalf("a buyer signing in on the console = %d, want 401", refused.Code)
	}
	store := c.onStore(t)
	if refused := store.signIn(ownerEmail, ownerPassword); refused.Code != http.StatusUnauthorized {
		t.Fatalf("the owner signing in on a store = %d, want 401", refused.Code)
	}

	owner := c.onConsole(t)
	wantRedirect(t, owner.signIn(ownerEmail, ownerPassword), "/pt-BR/")
	store.cookies = owner.cookies
	wantRedirect(t, store.get("/account/password"), "/pt-BR/signin")

	buyer := c.onStore(t)
	wantRedirect(t, buyer.signIn("leitora@example.test", ownerPassword), "/pt-BR/")
	onConsole := c.onConsole(t)
	onConsole.cookies = buyer.cookies
	wantRedirect(t, onConsole.get("/"), "/pt-BR/signin")
}

// The console's own names never answer on another host.
func TestTheConsoleIsNotServedOnAStore(t *testing.T) {
	c := newConsole(t, true)
	store := c.onStore(t)
	for _, path := range []string{"/console/version", "/enrol", "/setup"} {
		if answer := store.get(path); answer.Code != http.StatusNotFound {
			t.Errorf("%s on a store = %d, want 404", path, answer.Code)
		}
	}
}
