//go:build integration

package httpx_test

import (
	"html"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/aleogr/marketplace/internal/platform/httpx"
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
