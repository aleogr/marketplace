//go:build integration

package httpx_test

import (
	"crypto/hmac"
	"crypto/sha1" // #nosec G505 -- RFC 6238's HMAC-SHA-1, as the service computes it.
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/aleogr/marketplace/internal/identity"
	"github.com/aleogr/marketplace/internal/identity/breached"
	"github.com/aleogr/marketplace/internal/platform/audit"
	"github.com/aleogr/marketplace/internal/platform/httpx"
	"github.com/aleogr/marketplace/internal/platform/keys/local"
	"github.com/aleogr/marketplace/internal/platform/ratelimit"
	"github.com/aleogr/marketplace/internal/tenancy"
)

// appCodeFor is what an authenticator app set up with the key a page shows
// would show at now, computed here independently of the service: RFC 6238
// with SHA-1, six digits and thirty-second steps.
func appCodeFor(t *testing.T, key string, now time.Time) string {
	t.Helper()
	secret, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.ReplaceAll(key, " ", ""))
	if err != nil {
		t.Fatalf("the page's key %q is not base32: %v", key, err)
	}
	var counter [8]byte
	binary.BigEndian.PutUint64(counter[:], uint64(now.Unix()/30)) // #nosec G115 -- a test clock is after 1970.
	mac := hmac.New(sha1.New, secret)
	mac.Write(counter[:])
	sum := mac.Sum(nil)
	offset := sum[len(sum)-1] & 0x0f
	return fmt.Sprintf("%06d", (binary.BigEndian.Uint32(sum[offset:offset+4])&0x7fffffff)%1000000)
}

// keyOnPage is the key an app enrolment page shows.
var keyOnPage = regexp.MustCompile(`<code id="totp-key" class="key">([A-Z2-7 ]+)</code>`)

// securityHandler is the identity routes behind the session middleware, over
// a service that seals with keys of the test's own.
func securityHandler(t *testing.T) (http.Handler, *identity.Service, *tenancy.Marketplace) {
	t.Helper()
	pool, marketplace := migrated(t)
	keeper, err := local.Generate()
	if err != nil {
		t.Fatal(err)
	}
	service := identity.NewService(serving{t, pool}, identity.NewHasher(cheap, 2), breached.Fake{}, unaudited{}, silent()).
		WithSealer(audit.NewKeys(keeper))
	never := ratelimit.Never{}
	handler := httpx.Sessions(service, silent())(identityHandler(t, service, httpx.IdentityLimits{
		SignUp: never, Resend: never, ResendAddress: never, SignIn: never, SignInAddress: never, Password: never,
		StepUp: never,
	}))
	confirmedAccount(t, pool, service, marketplace, "Leitora", "leitora@example.test", "correct horse battery staple")
	return handler, service, marketplace
}

// browser sends requests to handler and keeps the cookies the responses set,
// as a browser does: the session's, and any other a page hands it.
type browser struct {
	t           *testing.T
	handler     http.Handler
	marketplace *tenancy.Marketplace
	cookies     map[string]string
}

func (b browser) get(path string) *httptest.ResponseRecorder {
	request := httptest.NewRequestWithContext(b.t.Context(), http.MethodGet, path, nil)
	return b.send(request)
}

func (b browser) post(path string, form url.Values) *httptest.ResponseRecorder {
	return b.send(formRequest(b.t, path, form))
}

func (b browser) send(request *http.Request) *httptest.ResponseRecorder {
	for name, value := range b.cookies {
		request.AddCookie(&http.Cookie{Name: name, Value: value})
	}
	recorder := httptest.NewRecorder()
	b.handler.ServeHTTP(recorder, in(request, b.marketplace, "203.0.113.10"))
	for _, set := range recorder.Result().Cookies() {
		if set.MaxAge < 0 {
			delete(b.cookies, set.Name)
			continue
		}
		b.cookies[set.Name] = set.Value
	}
	return recorder
}

// signedInBrowser signs the fixture's account in through the service.
func signedInBrowser(t *testing.T, handler http.Handler, service *identity.Service, marketplace *tenancy.Marketplace) browser {
	t.Helper()
	token, err := service.SignIn(t.Context(), identity.Visit{Marketplace: marketplace.ID, Language: "pt-BR"},
		"leitora@example.test", "correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	return browser{t: t, handler: handler, marketplace: marketplace, cookies: map[string]string{"session": token}}
}

// Adding an app through the pages: the QR code and the key, a wrong code
// refused with the same key shown again, the right one accepted with the ten
// recovery codes, the security page listing the app, and its removal.
func TestAddingAndRemovingAnAppThroughThePages(t *testing.T) {
	handler, service, marketplace := securityHandler(t)
	b := signedInBrowser(t, handler, service, marketplace)

	empty := b.get("/account/security")
	if empty.Code != http.StatusOK || !strings.Contains(empty.Body.String(), "A verificação em duas etapas está desligada") {
		t.Fatalf("the security page before any factor: status %d, body %s", empty.Code, empty.Body.String())
	}

	start := b.get("/account/security/app")
	body := start.Body.String()
	found := keyOnPage.FindStringSubmatch(body)
	if start.Code != http.StatusOK || found == nil || !strings.Contains(body, `src="data:image/png;base64,`) {
		t.Fatalf("the app page: status %d, body %s", start.Code, body)
	}
	key := found[1]

	wrong := b.post("/account/security/app", url.Values{"label": {"Celular"}, "code": {"000000"}})
	if wrong.Code != http.StatusUnprocessableEntity || !strings.Contains(wrong.Body.String(), "Esse código não confere.") {
		t.Fatalf("a wrong code: status %d, body %s", wrong.Code, wrong.Body.String())
	}
	if again := keyOnPage.FindStringSubmatch(wrong.Body.String()); again == nil || again[1] != key {
		t.Fatalf("after a wrong code the page shows another key: %v, want %q", again, key)
	}
	if !strings.Contains(wrong.Body.String(), `value="Celular"`) {
		t.Error("the label typed was not kept")
	}

	added := b.post("/account/security/app", url.Values{"label": {"Celular"}, "code": {appCodeFor(t, key, time.Now())}})
	if added.Code != http.StatusOK || strings.Count(added.Body.String(), "<li><code>") != identity.RecoveryCodeCount ||
		!strings.Contains(added.Body.String(), "O segundo fator foi adicionado.") {
		t.Fatalf("the right code: status %d, body %s", added.Code, added.Body.String())
	}

	listed := b.get("/account/security")
	listing := listed.Body.String()
	if !strings.Contains(listing, "<strong>Celular</strong>") || !strings.Contains(listing, "Restam 10 dos seus 10 códigos") {
		t.Fatalf("the security page does not list the app: %s", listing)
	}
	factor := regexp.MustCompile(`name="factor" value="([0-9a-f-]+)"`).FindStringSubmatch(listing)
	if factor == nil {
		t.Fatalf("the security page has no removal form: %s", listing)
	}

	// The account has 2FA now: removing the app asks for it again first
	// (D2), and comes back.
	asked := b.post("/account/security/remove", url.Values{"factor": {factor[1]}})
	if asked.Code != http.StatusSeeOther || asked.Header().Get("Location") != "/pt-BR/account/verify?for=factors&next=%2Faccount%2Fsecurity" {
		t.Fatalf("removal with no step-up: status %d, Location %q", asked.Code, asked.Header().Get("Location"))
	}
	stepUp := b.get("/account/verify?for=factors&next=%2Faccount%2Fsecurity")
	if stepUp.Code != http.StatusOK || !strings.Contains(stepUp.Body.String(), "Antes de alterar seus segundos fatores, confirme que é você.") {
		t.Fatalf("the step-up page: status %d, body %s", stepUp.Code, stepUp.Body.String())
	}
	back := b.post("/account/verify", url.Values{"for": {"factors"}, "next": {"/account/security"}, "method": {"totp"},
		"code": {appCodeFor(t, key, time.Now().Add(30*time.Second))}})
	if back.Code != http.StatusSeeOther || back.Header().Get("Location") != "/pt-BR/account/security" {
		t.Fatalf("the step-up: status %d, Location %q, body %s", back.Code, back.Header().Get("Location"), back.Body.String())
	}

	removed := b.post("/account/security/remove", url.Values{"factor": {factor[1]}})
	if removed.Code != http.StatusSeeOther || removed.Header().Get("Location") != "/pt-BR/account/security?done=removed" {
		t.Fatalf("removal: status %d, Location %q", removed.Code, removed.Header().Get("Location"))
	}
	if again := b.post("/account/security/remove", url.Values{"factor": {factor[1]}}); again.Code != http.StatusNotFound {
		t.Fatalf("removing it again: status %d, want %d", again.Code, http.StatusNotFound)
	}
	if after := b.get("/account/security?done=removed"); !strings.Contains(after.Body.String(), "O segundo fator foi removido.") ||
		!strings.Contains(after.Body.String(), "A verificação em duas etapas está desligada") {
		t.Fatalf("the security page after the removal: %s", after.Body.String())
	}
}

// A reload of the app page — a phone reloads a tab it discarded, and the
// page is not kept in the back-forward cache — shows the key the person may
// already have typed into their app, while its enrolment lives; once the
// enrolment expired, a new key, which is the one that confirms.
func TestReloadingTheAppPageKeepsItsKey(t *testing.T) {
	handler, service, marketplace := securityHandler(t)
	b := signedInBrowser(t, handler, service, marketplace)
	keyOf := func(page *httptest.ResponseRecorder) string {
		t.Helper()
		found := keyOnPage.FindStringSubmatch(page.Body.String())
		if page.Code != http.StatusOK || found == nil {
			t.Fatalf("the app page: status %d, body %s", page.Code, page.Body.String())
		}
		return found[1]
	}

	first := keyOf(b.get("/account/security/app"))
	if again := keyOf(b.get("/account/security/app")); again != first {
		t.Fatalf("a reload shows the key %q, want the one already shown, %q", again, first)
	}

	// The enrolment's lifetime passes: its expiry is moved back rather than
	// waited for.
	if _, err := poolOf(t).Exec(t.Context(), `
		UPDATE factor_enrolment SET expires_at = now() - interval '1 second' WHERE marketplace_id = $1`,
		marketplace.ID); err != nil {
		t.Fatal(err)
	}
	fresh := keyOf(b.get("/account/security/app"))
	if fresh == first {
		t.Fatalf("after the enrolment expired the page shows its key %q again", first)
	}
	added := b.post("/account/security/app", url.Values{"label": {"Celular"}, "code": {appCodeFor(t, fresh, time.Now())}})
	if added.Code != http.StatusOK || !strings.Contains(added.Body.String(), "O segundo fator foi adicionado.") {
		t.Fatalf("the new key's code: status %d, body %s", added.Code, added.Body.String())
	}
}

// A tampered or empty factor field is a factor the account does not have,
// not a database error: it must answer 404, not 500.
func TestRemovingAFactorByAMalformedIDIsNotFound(t *testing.T) {
	handler, service, marketplace := securityHandler(t)
	b := signedInBrowser(t, handler, service, marketplace)

	for name, id := range map[string]string{"malformed": "nope", "empty": ""} {
		t.Run(name, func(t *testing.T) {
			removed := b.post("/account/security/remove", url.Values{"factor": {id}})
			if removed.Code != http.StatusNotFound {
				t.Fatalf("factor=%q: status %d, want %d", id, removed.Code, http.StatusNotFound)
			}
		})
	}
}
