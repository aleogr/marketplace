//go:build integration

package httpx_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/aleogr/marketplace/internal/platform/httpx"
)

// enrolThroughPages adds an app from the security pages and returns its key.
func enrolThroughPages(t *testing.T, b browser) string {
	t.Helper()
	found := keyOnPage.FindStringSubmatch(b.get("/account/security/app").Body.String())
	if found == nil {
		t.Fatal("the app page shows no key")
	}
	added := b.post("/account/security/app", url.Values{"code": {appCodeFor(t, found[1], time.Now())}})
	if added.Code != http.StatusOK {
		t.Fatalf("adding the app: status %d", added.Code)
	}
	return found[1]
}

// A step-up returns only to a path on this site: an address that names
// another host is read as this site's home, never followed.
func TestAStepUpReturnsOnlyToThisSite(t *testing.T) {
	handler, service, marketplace := securityHandler(t)
	b := signedInBrowser(t, handler, service, marketplace)
	key := enrolThroughPages(t, b)

	if page := b.get("/account/verify?for=factors&next=%2F%2Fevil.example%2F"); page.Code != http.StatusOK {
		t.Fatalf("the step-up page: status %d", page.Code)
	}
	back := b.post("/account/verify", url.Values{"for": {"factors"}, "next": {"//evil.example/"}, "method": {"totp"},
		"code": {appCodeFor(t, key, time.Now().Add(30*time.Second))}})
	if back.Code != http.StatusSeeOther || back.Header().Get("Location") != "/pt-BR/" {
		t.Fatalf("status %d, Location %q; want %d to /pt-BR/", back.Code, back.Header().Get("Location"), http.StatusSeeOther)
	}
}

// A wrong answer to a step-up says so on the same page, which still offers
// the recovery codes.
func TestAWrongStepUpAnswerIsRefusedInThePage(t *testing.T) {
	handler, service, marketplace := securityHandler(t)
	b := signedInBrowser(t, handler, service, marketplace)
	enrolThroughPages(t, b)

	b.get("/account/verify?for=password&next=%2Faccount%2Fpassword")
	wrong := b.post("/account/verify", url.Values{"for": {"password"}, "next": {"/account/password"}, "method": {"totp"}, "code": {"000000"}})
	body := wrong.Body.String()
	if wrong.Code != http.StatusUnauthorized || !strings.Contains(body, "Esse código não confere. Tente de novo.") ||
		!strings.Contains(body, "Antes de alterar sua senha, confirme que é você.") || !strings.Contains(body, "Usar um código de recuperação") {
		t.Fatalf("a wrong step-up answer: status %d, body %s", wrong.Code, body)
	}
}

// A wrong answer to a step-up is a page posted to, whose address carries no
// query; the language switch on it still returns to the same step-up, for
// the same action and with the method it showed, rather than to a step-up
// page that no longer knows what it is for.
func TestTheLanguageSwitchAfterAWrongStepUpAnswerKeepsTheStepUp(t *testing.T) {
	handler, service, marketplace := bilingualSite(t)
	b := signedInBrowser(t, handler, service, marketplace)
	enrolledApp(t, service, marketplace)

	b.get("/pt-BR/account/verify?for=password&next=%2Faccount%2Fpassword")
	wrong := b.post("/pt-BR/account/verify", url.Values{"for": {"password"}, "next": {"/account/password"},
		"method": {"totp"}, "code": {"000000"}})
	body := wrong.Body.String()
	if wrong.Code != http.StatusUnauthorized {
		t.Fatalf("a wrong step-up answer: status %d, body %s", wrong.Code, body)
	}
	if query := hidden(t, queryOnPage, body); query != "for=password&method=totp&next=%2Faccount%2Fpassword" {
		t.Fatalf("the language switch carries the query %q", query)
	}

	switched := b.post(httpx.LanguagePath, url.Values{"language": {"en-US"},
		"path": {hidden(t, pathOnPage, body)}, "query": {hidden(t, queryOnPage, body)}})
	location := switched.Header().Get("Location")
	if switched.Code != http.StatusSeeOther || !strings.HasPrefix(location, "/en-US/account/verify?") {
		t.Fatalf("the switch: status %d, Location %q", switched.Code, location)
	}
	again := b.get(location)
	if again.Code != http.StatusOK || !strings.Contains(again.Body.String(), "Before changing your password, confirm it is you.") {
		t.Fatalf("the step-up page after the switch: status %d, body %s", again.Code, again.Body.String())
	}
}

// The same holds for the sign-in's second step: after a wrong code, the
// language switch returns to the second step with the method it showed.
func TestTheLanguageSwitchAfterAWrongSecondStepKeepsTheMethod(t *testing.T) {
	handler, service, marketplace := bilingualSite(t)
	enrolledApp(t, service, marketplace)
	visitor := browser{t: t, handler: handler, marketplace: marketplace, cookies: map[string]string{}}

	visitor.post("/pt-BR/signin", url.Values{"email": {"leitora@example.test"}, "password": {"correct horse battery staple"}})
	if visitor.cookies["challenge"] == "" {
		t.Fatal("the password opened no challenge")
	}
	wrong := visitor.post("/pt-BR/signin/verify", url.Values{"method": {"totp"}, "code": {"000000"}})
	body := wrong.Body.String()
	if wrong.Code != http.StatusUnauthorized {
		t.Fatalf("a wrong second step: status %d, body %s", wrong.Code, body)
	}
	if query := hidden(t, queryOnPage, body); query != "method=totp" {
		t.Fatalf("the language switch carries the query %q", query)
	}
	switched := visitor.post(httpx.LanguagePath, url.Values{"language": {"en-US"},
		"path": {hidden(t, pathOnPage, body)}, "query": {hidden(t, queryOnPage, body)}})
	if location := switched.Header().Get("Location"); switched.Code != http.StatusSeeOther || location != "/en-US/signin/verify?method=totp" {
		t.Fatalf("the switch: status %d, Location %q", switched.Code, location)
	}
}
