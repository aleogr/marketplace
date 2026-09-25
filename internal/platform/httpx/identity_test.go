package httpx_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

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
			SignIn: never, SignInAddress: never, Password: never, StepUp: never,
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
		{http.MethodGet, "/signin"}, {http.MethodPost, "/signin"}, {http.MethodPost, "/signout"},
		{http.MethodGet, "/account/password"}, {http.MethodPost, "/account/password"},
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

// refusing is a limiter that has had enough of everybody.
type refusing struct{}

func (refusing) Allow(context.Context, string) (ratelimit.Decision, error) {
	return ratelimit.Decision{RetryAfter: time.Minute}, nil
}

// limitsRefusing are the identity limits with slot refusing and every other
// limit allowing.
func limitsRefusing(slot string) httpx.IdentityLimits {
	pick := func(name string) ratelimit.Limiter {
		if name == slot {
			return refusing{}
		}
		return ratelimit.Never{}
	}
	return httpx.IdentityLimits{
		SignUp: pick("SignUp"), Resend: pick("Resend"), ResendAddress: pick("ResendAddress"),
		SignIn: pick("SignIn"), SignInAddress: pick("SignInAddress"), Password: pick("Password"),
		StepUp: pick("StepUp"),
	}
}

// signedIn is a request as it reaches the routes from a signed-in browser:
// the session middleware has put the account in its context.
func signedIn(r *http.Request) *http.Request {
	return r.WithContext(httpx.WithSession(r.Context(), identity.Session{
		ID:      "00000000-0000-0000-0000-00000000000a",
		Account: identity.Account{ID: "00000000-0000-0000-0000-00000000000b", Name: "Leitora", Kind: identity.KindBuyer},
	}))
}

// Each limiter guards the routes it is named for, and only those. The forms
// posted stop before the database whenever they are let through: a sign-up
// with no name, a resend or a sign-in for an address that is not one, a
// password change to a password too short to set.
func TestEachIdentityLimitGuardsItsOwnRoute(t *testing.T) {
	signUp := url.Values{"name": {""}, "email": {"a@example.test"}, "password": {"correct horse battery"}}
	resend := url.Values{"email": {"not an address"}}
	signIn := url.Values{"email": {"not an address"}, "password": {"correct horse battery"}}
	password := url.Values{"current_password": {"correct horse battery"}, "new_password": {"curta"}}
	guarded := map[string][]string{
		"SignUp":        {"/signup"},
		"Resend":        {"/verify/resend"},
		"ResendAddress": {"/verify/resend"},
		"SignIn":        {"/signin"},
		"SignInAddress": {"/signin"},
		"Password":      {"/account/password"},
	}
	forms := map[string]url.Values{
		"/signup": signUp, "/verify/resend": resend, "/signin": signIn, "/account/password": password,
	}
	for slot, routes := range guarded {
		handler := identityHandler(t,
			identity.NewService(nil, identity.NewHasher(cheap, 1), breached.Fake{}, nil, silent()),
			limitsRefusing(slot))
		for path, form := range forms {
			want := slices.Contains(routes, path)
			t.Run(slot+" POST "+path, func(t *testing.T) {
				recorder := httptest.NewRecorder()
				handler.ServeHTTP(recorder, signedIn(formRequest(t, path, form)))
				if refused := recorder.Code == http.StatusTooManyRequests; refused != want {
					t.Errorf("status = %d; refused = %v, want %v", recorder.Code, refused, want)
				}
			})
		}
	}
}

// Reading a form costs nothing and is never limited, even when every limit
// refuses.
func TestTheIdentityPagesAreNotLimited(t *testing.T) {
	all := refusing{}
	handler := identityHandler(t,
		identity.NewService(nil, identity.NewHasher(cheap, 1), breached.Fake{}, nil, silent()),
		httpx.IdentityLimits{SignUp: all, Resend: all, ResendAddress: all, SignIn: all, SignInAddress: all, Password: all})
	for _, path := range []string{"/signup", "/verify?token=x", "/verify/resend", "/signin", "/account/password"} {
		t.Run(path, func(t *testing.T) {
			request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil)
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, signedIn(inMarketplace(request)))
			if recorder.Code != http.StatusOK {
				t.Errorf("status = %d, want %d", recorder.Code, http.StatusOK)
			}
		})
	}
}

// The name's bound reaches the page from the rule, and so does its message.
func TestTheNameIsBoundedInThePageAndByTheRule(t *testing.T) {
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/signup", nil)
	recorder := httptest.NewRecorder()
	identitySite(t).ServeHTTP(recorder, inMarketplace(request))
	if want := `maxlength="` + strconv.Itoa(identity.MaxNameLength) + `"`; !strings.Contains(recorder.Body.String(), want) {
		t.Errorf("the sign-up page does not carry %s", want)
	}

	recorder = postForm(t, "/signup", url.Values{
		"name":  {strings.Repeat("a", identity.MaxNameLength+1)},
		"email": {"a@example.test"}, "password": {"correct horse battery"},
	})
	want := "O nome pode ter no máximo 100 caracteres, sem caracteres invisíveis ou de controle."
	if recorder.Code != http.StatusUnprocessableEntity || !strings.Contains(recorder.Body.String(), want) {
		t.Fatalf("status %d, body without %q: %s", recorder.Code, want, recorder.Body.String())
	}
}

// The password page is a signed-in page: anybody else is sent to sign in, and
// a form posted without a session is not counted against any account's limit
// (the refusing limiter would answer 429 if it were asked).
func TestThePasswordPageSendsASignedOutVisitorToSignIn(t *testing.T) {
	all := refusing{}
	handler := identityHandler(t,
		identity.NewService(nil, identity.NewHasher(cheap, 1), breached.Fake{}, nil, silent()),
		httpx.IdentityLimits{SignUp: all, Resend: all, ResendAddress: all, SignIn: all, SignInAddress: all, Password: all})
	form := url.Values{"current_password": {"correct horse battery"}, "new_password": {"a brand new passphrase"}}
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		t.Run(method, func(t *testing.T) {
			request := httptest.NewRequestWithContext(t.Context(), method, "/account/password",
				strings.NewReader(form.Encode()))
			request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, inMarketplace(request))
			if recorder.Code != http.StatusSeeOther || recorder.Header().Get("Location") != "/pt-BR/signin" {
				t.Errorf("status %d, Location %q; want %d to /pt-BR/signin",
					recorder.Code, recorder.Header().Get("Location"), http.StatusSeeOther)
			}
		})
	}
}

// The signed-in page asks for the current password and a new one bounded by
// the rule, and the header links to it.
func TestThePasswordPageAsksForBothPasswords(t *testing.T) {
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/account/password", nil)
	recorder := httptest.NewRecorder()
	identitySite(t).ServeHTTP(recorder, signedIn(inMarketplace(request)))

	body := recorder.Body.String()
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	for _, want := range []string{
		"<h1>Alterar senha</h1>",
		`action="/pt-BR/account/password"`,
		`name="current_password" type="password" autocomplete="current-password"`,
		`autocomplete="new-password"`,
		`minlength="` + strconv.Itoa(identity.MinPasswordLength) + `"`,
		`maxlength="` + strconv.Itoa(identity.MaxPasswordLength) + `"`,
		"No mínimo 12 caracteres.",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the password page does not carry %s", want)
		}
	}
	if nav := accountNav(body); !strings.Contains(nav, `href="/pt-BR/account/password"`) || !strings.Contains(nav, "Senha") {
		t.Errorf("the signed-in header does not link to the password page: %q", nav)
	}
}

// A new password the rule refuses is refused before the database, with the
// rule's message.
func TestThePasswordPageShowsTheRulesMessage(t *testing.T) {
	recorder := httptest.NewRecorder()
	identitySite(t).ServeHTTP(recorder, signedIn(formRequest(t, "/account/password",
		url.Values{"current_password": {"correct horse battery"}, "new_password": {"curta"}})))

	want := "A senha precisa de pelo menos 12 caracteres."
	if recorder.Code != http.StatusUnprocessableEntity || !strings.Contains(recorder.Body.String(), want) {
		t.Fatalf("status %d, body without %q: %s", recorder.Code, want, recorder.Body.String())
	}
}

// The page a password change redirects to says the change is done; the page
// itself, reached any other way, does not.
func TestThePasswordPageSaysTheChangeIsDoneOnlyAfterIt(t *testing.T) {
	get := func(path string) string {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil)
		recorder := httptest.NewRecorder()
		identitySite(t).ServeHTTP(recorder, signedIn(inMarketplace(request)))
		return recorder.Body.String()
	}
	const done = "Sua senha foi alterada."
	if body := get("/account/password?changed=1"); !strings.Contains(body, done) || !strings.Contains(body, `role="status"`) {
		t.Errorf("the page after a change does not say it is done: %s", body)
	}
	if body := get("/account/password"); strings.Contains(body, done) {
		t.Errorf("the page says a change is done before any: %s", body)
	}
}

// breachAware is identitySite over the fake breach list the suite shares, so a
// breached password is refused before the database too.
func breachAware(t *testing.T) http.Handler {
	t.Helper()
	never := ratelimit.Never{}
	return identityHandler(t,
		identity.NewService(nil, identity.NewHasher(cheap, 1), breached.Fake{Known: breached.Common}, nil, silent()),
		httpx.IdentityLimits{
			SignUp: never, Resend: never, ResendAddress: never,
			SignIn: never, SignInAddress: never, Password: never, StepUp: never,
		})
}

// input finds the tag of the input named name in body.
func input(t *testing.T, body, name string) string {
	t.Helper()
	tag := regexp.MustCompile(`<input[^>]*\sname="` + regexp.QuoteMeta(name) + `"[^>]*>`).FindString(body)
	if tag == "" {
		t.Fatalf("the page has no input named %s: %s", name, body)
	}
	return tag
}

// flagged reports how a tag says it is the one a refusal is about.
func flagged(tag string) (invalid, describedByError, focused bool) {
	describedBy := regexp.MustCompile(`\saria-describedby="([^"]*)"`).FindStringSubmatch(tag)
	return strings.Contains(tag, ` aria-invalid="true"`),
		describedBy != nil && slices.Contains(strings.Fields(describedBy[1]), "form-error"),
		regexp.MustCompile(`\sautofocus[\s/>=]`).MatchString(tag)
}

// A refusal is named, so the field it is about can point at it, and that
// field, and only that one, is marked invalid and takes the cursor: a visitor
// who does not see the sentence still lands where the fix is. The field is
// chosen by the handler from the refusal's key, never by the template.
func TestARefusalIsTiedToTheFieldItIsAbout(t *testing.T) {
	const good = "correct horse battery"
	for _, tc := range []struct {
		name, path string
		form       url.Values
		field      string
	}{
		{"sign-up without a name", "/signup",
			url.Values{"name": {""}, "email": {"a@example.test"}, "password": {good}}, "name"},
		{"sign-up with too long a name", "/signup",
			url.Values{"name": {strings.Repeat("a", identity.MaxNameLength+1)}, "email": {"a@example.test"}, "password": {good}}, "name"},
		{"sign-up with an invalid address", "/signup",
			url.Values{"name": {"Leitora"}, "email": {"not an address"}, "password": {good}}, "email"},
		{"sign-up with a short password", "/signup",
			url.Values{"name": {"Leitora"}, "email": {"a@example.test"}, "password": {"curta"}}, "password"},
		{"sign-up with too long a password", "/signup",
			url.Values{"name": {"Leitora"}, "email": {"a@example.test"}, "password": {strings.Repeat("a", identity.MaxPasswordLength+1)}}, "password"},
		{"sign-up with a breached password", "/signup",
			url.Values{"name": {"Leitora"}, "email": {"a@example.test"}, "password": {"password1234"}}, "password"},
		{"sign-in refused", "/signin",
			url.Values{"email": {"not an address"}, "password": {good}}, "password"},
		// A current password beyond the byte cap no settable password reaches
		// is refused as a wrong one before the database (identity, policy.go).
		{"password change with a wrong current password", "/account/password",
			url.Values{"current_password": {strings.Repeat("a", 16*identity.MaxPasswordLength+1)}, "new_password": {"a brand new passphrase"}}, "current_password"},
		{"password change to a short password", "/account/password",
			url.Values{"current_password": {good}, "new_password": {"curta"}}, "new_password"},
		{"password change to too long a password", "/account/password",
			url.Values{"current_password": {good}, "new_password": {strings.Repeat("a", identity.MaxPasswordLength+1)}}, "new_password"},
		{"password change to a breached password", "/account/password",
			url.Values{"current_password": {good}, "new_password": {"password1234"}}, "new_password"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			breachAware(t).ServeHTTP(recorder, signedIn(formRequest(t, tc.path, tc.form)))
			body := recorder.Body.String()
			if recorder.Code < 400 || recorder.Code >= 500 {
				t.Fatalf("status = %d, want a refusal: %s", recorder.Code, body)
			}
			if !regexp.MustCompile(`<p[^>]*\sid="form-error"[^>]*\srole="alert"|<p[^>]*\srole="alert"[^>]*\sid="form-error"`).MatchString(body) {
				t.Errorf("the refusal is not named form-error: %s", body)
			}
			for name := range tc.form {
				invalid, described, focused := flagged(input(t, body, name))
				want := name == tc.field
				if invalid != want || described != want || focused != want {
					t.Errorf("%s: aria-invalid %v, described by the refusal %v, autofocus %v; want %v for each",
						name, invalid, described, focused, want)
				}
			}
		})
	}
}

// A form nobody has posted is refused about nothing: no field is marked, none
// takes the cursor, and there is no refusal to point at.
func TestAFreshFormMarksNoField(t *testing.T) {
	for path, names := range map[string][]string{
		"/signup":           {"name", "email", "password"},
		"/signin":           {"email", "password"},
		"/account/password": {"current_password", "new_password"},
		"/verify/resend":    {"email"},
	} {
		t.Run(path, func(t *testing.T) {
			request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil)
			recorder := httptest.NewRecorder()
			identitySite(t).ServeHTTP(recorder, signedIn(inMarketplace(request)))
			body := recorder.Body.String()
			if regexp.MustCompile(`<[a-z]+[^<>]*\s(role="alert"|id="form-error")`).MatchString(body) {
				t.Errorf("a fresh form carries a refusal: %s", body)
			}
			for _, name := range names {
				if invalid, described, focused := flagged(input(t, body, name)); invalid || described || focused {
					t.Errorf("%s: aria-invalid %v, described by a refusal %v, autofocus %v on a fresh form",
						name, invalid, described, focused)
				}
			}
		})
	}
}

// The password hint is the new password's description, refused or not, so a
// screen reader reads the rule with the field.
func TestThePasswordHintDescribesTheNewPassword(t *testing.T) {
	get := func(path string) string {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil)
		recorder := httptest.NewRecorder()
		identitySite(t).ServeHTTP(recorder, signedIn(inMarketplace(request)))
		return recorder.Body.String()
	}
	describedBy := regexp.MustCompile(`\saria-describedby="([^"]*)"`)
	for path, name := range map[string]string{"/signup": "password", "/account/password": "new_password"} {
		t.Run(path, func(t *testing.T) {
			body := get(path)
			if !strings.Contains(body, `<p id="password-hint">No mínimo 12 caracteres.`) {
				t.Errorf("the hint is not named password-hint: %s", body)
			}
			ids := describedBy.FindStringSubmatch(input(t, body, name))
			if ids == nil || !slices.Contains(strings.Fields(ids[1]), "password-hint") {
				t.Errorf("%s is not described by the hint: %s", name, input(t, body, name))
			}
		})
	}

	// Refused, it is described by the refusal first and by the hint still.
	recorder := postForm(t, "/signup", url.Values{"name": {"Leitora"}, "email": {"a@example.test"}, "password": {"curta"}})
	ids := describedBy.FindStringSubmatch(input(t, recorder.Body.String(), "password"))
	if ids == nil || strings.Join(strings.Fields(ids[1]), " ") != "form-error password-hint" {
		t.Errorf("a refused password is described by %q, want the refusal then the hint", ids)
	}
}
