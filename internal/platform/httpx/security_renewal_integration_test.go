//go:build integration

package httpx_test

import (
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/aleogr/marketplace/internal/identity"
)

// codeOnPage matches one recovery code as the recovery page lists it.
var codeOnPage = regexp.MustCompile(`<li><code>([^<]+)</code></li>`)

// codesOnPage are the recovery codes a page shows, in the order they appear.
func codesOnPage(t *testing.T, body string) []string {
	t.Helper()
	matches := codeOnPage.FindAllStringSubmatch(body, -1)
	codes := make([]string, len(matches))
	for i, m := range matches {
		codes[i] = m[1]
	}
	return codes
}

// Renewing the recovery codes shows a fresh set of ten, never cached, and the
// old set — the one the account's first factor issued — is no longer among
// them: it was replaced, not extended (spec, D6).
func TestRenewingRecoveryCodesReplacesThem(t *testing.T) {
	handler, service, marketplace := securityHandler(t)
	b := signedInBrowser(t, handler, service, marketplace)

	start := b.get("/account/security/app")
	found := keyOnPage.FindStringSubmatch(start.Body.String())
	if start.Code != http.StatusOK || found == nil {
		t.Fatalf("the app page: status %d, body %s", start.Code, start.Body.String())
	}
	key := found[1]

	added := b.post("/account/security/app", url.Values{"label": {"Celular"}, "code": {appCodeFor(t, key, time.Now())}})
	if added.Code != http.StatusOK {
		t.Fatalf("adding the app: status %d, body %s", added.Code, added.Body.String())
	}
	issued := codesOnPage(t, added.Body.String())
	if len(issued) != identity.RecoveryCodeCount {
		t.Fatalf("the app enrolment showed %d codes, want %d", len(issued), identity.RecoveryCodeCount)
	}

	renewed := b.post("/account/security/recovery", url.Values{})
	if renewed.Code != http.StatusOK {
		t.Fatalf("renewing the codes: status %d, body %s", renewed.Code, renewed.Body.String())
	}
	if got := renewed.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("renewing the codes: Cache-Control = %q, want %q", got, "no-store")
	}
	fresh := codesOnPage(t, renewed.Body.String())
	if len(fresh) != identity.RecoveryCodeCount {
		t.Fatalf("the renewal page showed %d codes, want %d", len(fresh), identity.RecoveryCodeCount)
	}
	if slices.Contains(fresh, issued[0]) {
		t.Fatal("the renewed set still holds the first code the app enrolment issued")
	}

	after := b.get("/account/security")
	if !strings.Contains(after.Body.String(), "Restam 10 dos seus 10 códigos") {
		t.Fatalf("the security page after the renewal does not count 10 of 10 left: %s", after.Body.String())
	}
}

// An account with no second factor has no recovery codes to renew: the page
// sends it back to the security page rather than showing an empty set.
func TestRenewingRecoveryCodesWithNoFactorGoesBackToSecurity(t *testing.T) {
	handler, service, marketplace := securityHandler(t)
	b := signedInBrowser(t, handler, service, marketplace)

	renewed := b.post("/account/security/recovery", url.Values{})
	if renewed.Code != http.StatusSeeOther {
		t.Fatalf("renewing with no factor: status %d, want %d", renewed.Code, http.StatusSeeOther)
	}
	if got := renewed.Header().Get("Location"); got != "/pt-BR/account/security" {
		t.Fatalf("renewing with no factor: Location = %q, want %q", got, "/pt-BR/account/security")
	}
}
