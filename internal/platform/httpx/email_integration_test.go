//go:build integration

package httpx_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/aleogr/marketplace/internal/platform/config"
	"github.com/aleogr/marketplace/internal/platform/db"
	"github.com/aleogr/marketplace/internal/platform/dbtest"
	"github.com/aleogr/marketplace/internal/tenancy"
)

// poolOf opens the package's database as its owner, for what a test reads or
// moves behind the service's back.
func poolOf(t *testing.T) *db.Pool {
	t.Helper()
	pool, err := db.Open(t.Context(), config.Database{URL: dbtest.URL(t)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// mailedCode is the code of the last second-factor-code mail requested in a
// marketplace, read from the outbox as the owner, before it is dispatched.
func mailedCode(t *testing.T, pool *db.Pool, marketplace *tenancy.Marketplace) string {
	t.Helper()
	var code string
	if err := pool.QueryRow(t.Context(), `
		SELECT payload->'Variables'->>'Code' FROM outbox_event
		 WHERE kind = 'email.send' AND payload->>'Template' = 'second-factor-code' AND payload->>'Marketplace' = $1
		 ORDER BY created_at DESC, id DESC LIMIT 1`, marketplace.ID).Scan(&code); err != nil {
		t.Fatalf("no code was mailed: %v", err)
	}
	return code
}

// E-mail added through the pages, then the e-mail path of the second step:
// the code asked for, refused within a minute of the last, sent, and typed.
func TestAddingEmailAndSigningInWithItThroughThePages(t *testing.T) {
	handler, service, marketplace := securityHandler(t)
	pool := poolOf(t)
	b := signedInBrowser(t, handler, service, marketplace)

	if page := b.get("/account/security/email"); page.Code != http.StatusOK || !strings.Contains(page.Body.String(), "leitora@example.test") {
		t.Fatalf("the e-mail page: status %d, body %s", page.Code, page.Body.String())
	}
	sent := b.post("/account/security/email/send", url.Values{})
	if sent.Code != http.StatusSeeOther || sent.Header().Get("Location") != "/pt-BR/account/security/email?sent=1" {
		t.Fatalf("sending the code: status %d, Location %q", sent.Code, sent.Header().Get("Location"))
	}
	if wrong := b.post("/account/security/email", url.Values{"code": {"12345x"}}); wrong.Code != http.StatusUnprocessableEntity ||
		!strings.Contains(wrong.Body.String(), "Esse código não confere ou expirou.") {
		t.Fatalf("a wrong code: status %d, body %s", wrong.Code, wrong.Body.String())
	}
	added := b.post("/account/security/email", url.Values{"code": {mailedCode(t, pool, marketplace)}})
	if added.Code != http.StatusSeeOther || added.Header().Get("Location") != "/pt-BR/account/security?done=added" {
		t.Fatalf("adding e-mail: status %d, Location %q", added.Code, added.Header().Get("Location"))
	}

	out := httptest.NewRecorder()
	password := formRequest(t, "/signin", url.Values{"email": {"leitora@example.test"}, "password": {"correct horse battery staple"}})
	handler.ServeHTTP(out, in(password, marketplace, "203.0.113.12"))
	visitor := browser{t: t, handler: handler, marketplace: marketplace, cookies: map[string]string{}}
	for _, c := range out.Result().Cookies() {
		visitor.cookies[c.Name] = c.Value
	}
	if visitor.cookies["challenge"] == "" {
		t.Fatalf("the password opened no challenge: status %d", out.Code)
	}
	step := visitor.get("/signin/verify")
	if !strings.Contains(step.Body.String(), "Enviamos um código de seis dígitos para seu e-mail.") {
		t.Fatalf("the second step does not offer the e-mail code: %s", step.Body.String())
	}
	tooSoon := visitor.post("/signin/verify/email", url.Values{})
	if tooSoon.Code != http.StatusTooManyRequests || !strings.Contains(tooSoon.Body.String(), "Enviamos um código há menos de um minuto.") {
		t.Fatalf("a code within a minute of the last: status %d, body %s", tooSoon.Code, tooSoon.Body.String())
	}
	// A minute passes: the enrolment's code is moved back rather than waited
	// for.
	if _, err := pool.Exec(t.Context(), `
		UPDATE email_code SET created_at = created_at - interval '2 minutes' WHERE marketplace_id = $1`,
		marketplace.ID); err != nil {
		t.Fatal(err)
	}
	if again := visitor.post("/signin/verify/email", url.Values{}); again.Code != http.StatusSeeOther ||
		again.Header().Get("Location") != "/pt-BR/signin/verify?method=email&sent=1" {
		t.Fatalf("sending the sign-in code: status %d, Location %q", again.Code, again.Header().Get("Location"))
	}
	signed := visitor.post("/signin/verify", url.Values{"method": {"email"}, "code": {mailedCode(t, pool, marketplace)}})
	if signed.Code != http.StatusSeeOther || signed.Header().Get("Location") != "/pt-BR/" || visitor.cookies["session"] == "" {
		t.Fatalf("the e-mail code: status %d, Location %q", signed.Code, signed.Header().Get("Location"))
	}
}

// An e-mail code answered for a card, on an account whose factor is an app,
// does not let the app be removed: the removal still asks for the app.
func TestAnEmailCodeForACardDoesNotRemoveTheApp(t *testing.T) {
	handler, service, marketplace := securityHandler(t)
	pool := poolOf(t)
	b := signedInBrowser(t, handler, service, marketplace)
	enrolThroughPages(t, b)

	if page := b.get("/account/verify?for=add_card&next=%2Faccount%2Fsecurity"); page.Code != http.StatusOK {
		t.Fatalf("the step-up page for a card: status %d", page.Code)
	}
	sent := b.post("/account/verify/email", url.Values{"for": {"add_card"}, "next": {"/account/security"}})
	if sent.Code != http.StatusSeeOther {
		t.Fatalf("sending the step-up code: status %d", sent.Code)
	}
	confirmed := b.post("/account/verify", url.Values{"for": {"add_card"}, "next": {"/account/security"}, "method": {"email"},
		"code": {mailedCode(t, pool, marketplace)}})
	if confirmed.Code != http.StatusSeeOther || confirmed.Header().Get("Location") != "/pt-BR/account/security" {
		t.Fatalf("the e-mail code for a card: status %d, Location %q", confirmed.Code, confirmed.Header().Get("Location"))
	}

	factor := regexp.MustCompile(`name="factor" value="([0-9a-f-]+)"`).FindStringSubmatch(b.get("/account/security").Body.String())
	if factor == nil {
		t.Fatal("the security page has no removal form")
	}
	asked := b.post("/account/security/remove", url.Values{"factor": {factor[1]}})
	if asked.Code != http.StatusSeeOther || asked.Header().Get("Location") != "/pt-BR/account/verify?for=factors&next=%2Faccount%2Fsecurity" {
		t.Fatalf("removal after an e-mail code for a card: status %d, Location %q", asked.Code, asked.Header().Get("Location"))
	}
}
