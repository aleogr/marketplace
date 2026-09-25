//go:build integration

package httpx_test

import (
	"crypto/rand"
	"encoding/base64"
	"html"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/aleogr/marketplace/internal/platform/httpx"
	"github.com/aleogr/marketplace/internal/tenancy"
)

// The key's page asks the browser for a key of this marketplace's host, with
// no attestation, through the site's one script, loaded under the response's
// own nonce: no inline script, nothing from another origin (F14 spec, D1).
func TestTheKeyPageAsksTheBrowserThroughTheSitesScript(t *testing.T) {
	handler, service, marketplace := securityHandler(t)
	b := signedInBrowser(t, handler, service, marketplace)
	b.handler = httpx.Secure(false)(handler)

	page := b.get("/account/security/key")
	body := page.Body.String()
	if page.Code != http.StatusOK || !strings.Contains(body, `data-webauthn="create"`) {
		t.Fatalf("the key page: status %d, body %s", page.Code, body)
	}
	options := html.UnescapeString(regexp.MustCompile(`data-options="([^"]*)"`).FindStringSubmatch(body)[1])
	for _, want := range []string{`"rp":{"name":"Loja","id":"example.com"}`, `"attestation":"none"`, `"userVerification":"preferred"`} {
		if !strings.Contains(options, want) {
			t.Errorf("the options %s do not carry %s", options, want)
		}
	}

	nonce := regexp.MustCompile(`'nonce-([^']+)'`).FindStringSubmatch(page.Header().Get("Content-Security-Policy"))
	scripts := regexp.MustCompile(`<script[^>]*>`).FindAllString(body, -1)
	if nonce == nil || len(scripts) != 1 || scripts[0] != `<script src="/assets/webauthn.js" nonce="`+nonce[1]+`" defer>` {
		t.Fatalf("the page's scripts are %v, want the site's one script under the nonce %v", scripts, nonce)
	}

	refused := b.post("/account/security/key", url.Values{"label": {"YubiKey"}, "credential": {"not a credential"}})
	if refused.Code != http.StatusUnprocessableEntity ||
		!strings.Contains(refused.Body.String(), "Não foi possível verificar a resposta da chave.") ||
		!strings.Contains(refused.Body.String(), `value="YubiKey"`) {
		t.Fatalf("a refused key: status %d, body %s", refused.Code, refused.Body.String())
	}
}

// storedKey gives the fixture's account a key, written as the application
// role: what these tests read is the page that asks for it, not the
// registration, which internal/identity tests with a software key. It
// returns the credential's id as the options carry it.
func storedKey(t *testing.T, marketplace *tenancy.Marketplace) string {
	t.Helper()
	id := make([]byte, 16)
	if _, err := rand.Read(id); err != nil {
		t.Fatal(err)
	}
	// The public key is never read on the way to the options, and the flags
	// are a key's at registration: user present and verified.
	if err := (serving{t, poolOf(t)}).InTxFor(t.Context(), marketplace.ID, func(tx pgx.Tx) error {
		_, err := tx.Exec(t.Context(), `
			INSERT INTO second_factor (account_id, marketplace_id, kind, label, credential_id, public_key,
			                           sign_count, credential_flags)
			SELECT id, marketplace_id, 'webauthn', 'YubiKey', $1, $2, 0, 5
			  FROM account WHERE email = 'leitora@example.test'`, id, []byte{0xa5})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return base64.RawURLEncoding.EncodeToString(id)
}

// A key answers a step-up: for an account whose factor is a key and whose
// session has no recent step-up, the step-up page asks the browser for that
// key, with the options its script needs.
func TestTheStepUpPageAsksForTheKey(t *testing.T) {
	handler, service, marketplace := bilingualSite(t)
	b := signedInBrowser(t, handler, service, marketplace)
	credential := storedKey(t, marketplace)

	page := b.get("/pt-BR/account/verify?for=factors&next=%2Faccount%2Fsecurity")
	body := page.Body.String()
	if page.Code != http.StatusOK || !strings.Contains(body, `data-webauthn="get"`) {
		t.Fatalf("the step-up page for a key: status %d, body %s", page.Code, body)
	}
	found := regexp.MustCompile(`data-options="([^"]*)"`).FindStringSubmatch(body)
	if found == nil {
		t.Fatalf("the step-up page carries no options: %s", body)
	}
	options := html.UnescapeString(found[1])
	for _, want := range []string{`"challenge":"`, `"rpId":"example.com"`, `"id":"` + credential + `"`} {
		if !strings.Contains(options, want) {
			t.Errorf("the options %s do not carry %s", options, want)
		}
	}
}
