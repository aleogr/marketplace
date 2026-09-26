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

// keyButton is the submit button of the form that answers with a key, or
// adds one.
var keyButton = regexp.MustCompile(`(?s)<form[^>]*data-webauthn=.*?(<button type="submit"[^>]*>)`)

// Without the script a key cannot answer, so the button that would post an
// empty answer, and spend one of the challenge's attempts, is hidden until
// the script shows it; the page's other methods stay one link away.
func TestTheKeyButtonWaitsForTheScript(t *testing.T) {
	handler, service, marketplace := bilingualSite(t)
	b := signedInBrowser(t, handler, service, marketplace)
	button := func(path string) string {
		t.Helper()
		page := b.get(path)
		found := keyButton.FindStringSubmatch(page.Body.String())
		if page.Code != http.StatusOK || found == nil {
			t.Fatalf("%s: status %d, no key form: %s", path, page.Code, page.Body.String())
		}
		return found[1]
	}

	if adding := button("/pt-BR/account/security/key"); !strings.Contains(adding, " hidden") {
		t.Errorf("the button that adds a key, %s, is shown before the script runs", adding)
	}
	storedKey(t, marketplace)
	if answering := button("/pt-BR/account/verify?for=factors&next=%2Faccount%2Fsecurity"); !strings.Contains(answering, " hidden") {
		t.Errorf("the button that answers with the key, %s, is shown before the script runs", answering)
	}
}

// The key's messages, as each language words them.
var (
	keyUnsupported = map[string]string{
		"pt-BR": "Este navegador não consegue usar uma chave de segurança nem a digital, o rosto ou o PIN deste dispositivo. Escolha outro jeito.",
		"en-US": "This browser cannot use a security key or this device's fingerprint, face or PIN. Choose another way.",
	}
	keyNeedsScript = map[string]string{
		"pt-BR": "A chave de segurança precisa de JavaScript, que está desligado neste navegador. Ative-o ou escolha outro jeito.",
		"en-US": "A security key needs JavaScript, which is off in this browser. Turn it on, or choose another way.",
	}
	keyEnrolNeedsScript = map[string]string{
		"pt-BR": "Adicionar uma chave precisa de JavaScript neste navegador.",
		"en-US": "Adding a key needs JavaScript in this browser.",
	}
)

// Every page that uses a key carries, hidden, the message the script shows
// when the browser has no WebAuthn — not "try again", which cannot help — and
// the one it shows when a ceremony fails; and what a browser with no script
// reads instead, in each language.
func TestTheKeyPagesCarryTheirMessagesHidden(t *testing.T) {
	handler, service, marketplace := bilingualSite(t)
	b := signedInBrowser(t, handler, service, marketplace)
	failed := map[string]string{
		"pt-BR": "O navegador não concluiu. Tente de novo ou escolha outro jeito.",
		"en-US": "The browser did not finish. Try again, or choose another way.",
	}
	check := func(path, language string) {
		t.Helper()
		page := b.get(path)
		body := page.Body.String()
		if page.Code != http.StatusOK {
			t.Fatalf("%s: status %d, body %s", path, page.Code, body)
		}
		for _, want := range []string{
			`<p id="key-unsupported" role="alert" hidden>` + html.EscapeString(keyUnsupported[language]) + `</p>`,
			`<p id="key-failed" role="alert" hidden>` + html.EscapeString(failed[language]) + `</p>`,
			`<noscript><p>` + html.EscapeString(keyNeedsScript[language]) + `</p></noscript>`,
		} {
			if !strings.Contains(body, want) {
				t.Errorf("%s does not carry %s: %s", path, want, body)
			}
		}
	}
	for _, language := range []string{"pt-BR", "en-US"} {
		check("/"+language+"/account/security/key", language)
	}
	storedKey(t, marketplace)
	for _, language := range []string{"pt-BR", "en-US"} {
		check("/"+language+"/account/verify?for=factors&next=%2Faccount%2Fsecurity", language)
	}
}

// Without the script, Enter in the label's field posts the form with no
// credential: the page says that adding a key needs JavaScript, rather than
// that the key's answer could not be verified, and keeps the label.
func TestAKeyAddedWithoutTheScriptSaysItNeedsIt(t *testing.T) {
	handler, service, marketplace := bilingualSite(t)
	b := signedInBrowser(t, handler, service, marketplace)
	for _, language := range []string{"pt-BR", "en-US"} {
		b.get("/" + language + "/account/security/key")
		refused := b.post("/"+language+"/account/security/key", url.Values{"label": {"YubiKey"}, "credential": {""}})
		body := refused.Body.String()
		if refused.Code != http.StatusUnprocessableEntity ||
			!strings.Contains(body, `role="alert">`+html.EscapeString(keyEnrolNeedsScript[language])+`</p>`) ||
			!strings.Contains(body, `value="YubiKey"`) {
			t.Fatalf("a key posted with no credential in %s: status %d, body %s", language, refused.Code, body)
		}
		if strings.Contains(body, "Não foi possível verificar") || strings.Contains(body, "could not be verified") {
			t.Fatalf("a key posted with no credential in %s reads as a refused answer: %s", language, body)
		}
	}
}
