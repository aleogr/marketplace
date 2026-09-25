//go:build integration

package httpx_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
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
