//go:build integration

package httpx_test

import (
	"html"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/aleogr/marketplace/internal/identity"
	"github.com/aleogr/marketplace/internal/tenancy"
)

// The lock's messages, as each language words them: what still works, and
// what lifts the lock (F14 spec, D8).
var (
	codesLocked = map[string]string{
		"pt-BR": "Houve respostas erradas demais nesta conta, então códigos de aplicativo autenticador ou por e-mail não funcionam mais. Confirmar com uma chave de segurança ou um código de recuperação, ou alterar a senha, faz com que voltem a funcionar.",
		"en-US": "Too many wrong answers were given for this account, so codes from an authenticator app or by e-mail no longer work. Confirming with a security key or a recovery code, or changing the password, makes them work again.",
	}
	wrongCode = map[string]string{
		"pt-BR": "Esse código não confere.",
		"en-US": "That code is not right.",
	}
	nothingLeft = map[string]string{
		"pt-BR": "Esta conta não tem chave de segurança nem código de recuperação restante",
		"en-US": "This account has no security key and no recovery code left",
	}
)

// noscriptOf finds what a page shows a browser with no script.
var noscriptOf = regexp.MustCompile(`(?s)<noscript>(.*?)</noscript>`)

// codeNames are how each language names the codes the lock refuses.
var codeNames = map[string]struct{ app, email string }{
	"pt-BR": {"aplicativo autenticador", "por e-mail"},
	"en-US": {"authenticator app", "by e-mail"},
}

// lockCodes gives the fixture's account as many failed second factors in a
// row as lock its codes, written as the database's owner: a hundred
// challenges are internal/identity's to test.
func lockCodes(t *testing.T, marketplace *tenancy.Marketplace) {
	t.Helper()
	if _, err := poolOf(t).Exec(t.Context(), `
		INSERT INTO second_factor_failure (account_id, marketplace_id, failures, updated_at)
		SELECT id, marketplace_id, $2, now() FROM account WHERE marketplace_id = $1 AND email = 'leitora@example.test'`,
		marketplace.ID, identity.FailuresLocked); err != nil {
		t.Fatal(err)
	}
}

// passwordDone is a visitor who typed the fixture's password in language and
// holds the challenge it opened.
func passwordDone(t *testing.T, handler http.Handler, marketplace *tenancy.Marketplace, language string) browser {
	t.Helper()
	visitor := browser{t: t, handler: handler, marketplace: marketplace, cookies: map[string]string{}}
	visitor.post("/"+language+"/signin", url.Values{"email": {"leitora@example.test"}, "password": {"correct horse battery staple"}})
	if visitor.cookies["challenge"] == "" {
		t.Fatal("the password opened no challenge")
	}
	return visitor
}

// A locked account's second step says why the codes are gone, in each
// language, and offers what still works: the key first, and a recovery code.
// An app code posted anyway is refused as locked, not as wrong.
func TestALockedSecondStepSaysSoAndOffersWhatStillWorks(t *testing.T) {
	handler, service, marketplace := bilingualSite(t)
	enrolledApp(t, service, marketplace)
	storedKey(t, marketplace)
	lockCodes(t, marketplace)

	for language, want := range map[string]struct{ key, recovery, app string }{
		"pt-BR": {"Use sua chave de segurança", "Usar um código de recuperação", "Usar o aplicativo autenticador"},
		"en-US": {"Use your security key", "Use a recovery code", "Use your authenticator app"},
	} {
		visitor := passwordDone(t, handler, marketplace, language)
		page := visitor.get("/" + language + "/signin/verify")
		body := page.Body.String()
		if page.Code != http.StatusOK || !strings.Contains(body, codesLocked[language]) ||
			!strings.Contains(body, want.key) || !strings.Contains(body, want.recovery) {
			t.Fatalf("the locked second step in %s: status %d, body %s", language, page.Code, body)
		}
		if strings.Contains(body, want.app) || strings.Contains(body, `inputmode="numeric"`) || strings.Contains(body, nothingLeft[language]) {
			t.Fatalf("the locked second step in %s still offers a code, or says nothing is left: %s", language, body)
		}
		// Nor does what a browser with no script reads send the person to a
		// code the lock refuses.
		if noscript := noscriptOf.FindStringSubmatch(body); noscript == nil ||
			strings.Contains(noscript[1], codeNames[language].app) || strings.Contains(noscript[1], codeNames[language].email) {
			t.Fatalf("the locked second step's text without a script in %s suggests a code: %v", language, noscript)
		}

		refused := visitor.post("/"+language+"/signin/verify", url.Values{"method": {"totp"}, "code": {"000000"}})
		body = refused.Body.String()
		if refused.Code != http.StatusForbidden || !strings.Contains(body, codesLocked[language]) || strings.Contains(body, wrongCode[language]) {
			t.Fatalf("an app code on the locked second step in %s: status %d, body %s", language, refused.Code, body)
		}
	}
}

// With no key and no recovery code left, the locked second step says that
// nothing is left to confirm with, and shows no form to answer.
func TestALockedSecondStepWithNothingLeftSaysSo(t *testing.T) {
	handler, service, marketplace := bilingualSite(t)
	enrolledApp(t, service, marketplace)
	if _, err := poolOf(t).Exec(t.Context(), `
		UPDATE recovery_code SET used_at = now()
		 WHERE account_id = (SELECT id FROM account WHERE marketplace_id = $1 AND email = 'leitora@example.test')`,
		marketplace.ID); err != nil {
		t.Fatal(err)
	}
	lockCodes(t, marketplace)

	for _, language := range []string{"pt-BR", "en-US"} {
		visitor := passwordDone(t, handler, marketplace, language)
		page := visitor.get("/" + language + "/signin/verify")
		body := page.Body.String()
		if page.Code != http.StatusOK || !strings.Contains(body, codesLocked[language]) || !strings.Contains(body, nothingLeft[language]) {
			t.Fatalf("the locked second step with nothing left in %s: status %d, body %s", language, page.Code, body)
		}
		if strings.Contains(body, `action="/`+language+`/signin/verify"`) || strings.Contains(body, `name="code"`) {
			t.Fatalf("the locked second step with nothing left in %s shows a form to answer: %s", language, body)
		}
	}
}

// The step-up is the same page, and is locked the same way: with neither an
// app code nor a code to the address, the recovery code is what it asks for.
func TestALockedStepUpSaysSo(t *testing.T) {
	handler, service, marketplace := bilingualSite(t)
	b := signedInBrowser(t, handler, service, marketplace)
	enrolledApp(t, service, marketplace)
	lockCodes(t, marketplace)

	page := b.get("/pt-BR/account/verify?for=add_card&next=%2Faccount%2Fsecurity")
	body := page.Body.String()
	if page.Code != http.StatusOK || !strings.Contains(body, codesLocked["pt-BR"]) ||
		!strings.Contains(body, "Digite um dos seus códigos de recuperação.") ||
		strings.Contains(body, "Usar o aplicativo autenticador") || strings.Contains(body, "Receber um código por e-mail") {
		t.Fatalf("the locked step-up: status %d, body %s", page.Code, body)
	}
	refused := b.post("/pt-BR/account/verify", url.Values{"method": {"totp"}, "code": {"000000"},
		"for": {"add_card"}, "next": {"/account/security"}})
	if refused.Code != http.StatusForbidden || !strings.Contains(refused.Body.String(), codesLocked["pt-BR"]) ||
		strings.Contains(refused.Body.String(), wrongCode["pt-BR"]) {
		t.Fatalf("an app code on the locked step-up: status %d, body %s", refused.Code, refused.Body.String())
	}
}

// storedEmailFactor gives the fixture's account e-mail as a second factor,
// written as the application role: what these tests read is the page, and
// adding e-mail is internal/identity's to test.
func storedEmailFactor(t *testing.T, marketplace *tenancy.Marketplace) {
	t.Helper()
	if err := (serving{t, poolOf(t)}).InTxFor(t.Context(), marketplace.ID, func(tx pgx.Tx) error {
		_, err := tx.Exec(t.Context(), `
			INSERT INTO second_factor (account_id, marketplace_id, kind, label)
			SELECT id, marketplace_id, 'email', '' FROM account WHERE email = 'leitora@example.test'`)
		return err
	}); err != nil {
		t.Fatal(err)
	}
}

// The lock's message is a status when the page is only shown, and an alert
// when the page answers what the lock refused: an app code posted, or a code
// asked for by e-mail from a page opened before the lock, which is answered
// with the page and a 403 rather than a 404. At the second step and at a
// step-up, in each language. Shown, it is a notice, never the success box
// (a plain status) with its check mark: a lock is a warning, and the layout
// gives the notice colours of its own.
func TestTheLockIsAnAlertWhenItRefuses(t *testing.T) {
	handler, service, marketplace := bilingualSite(t)
	// Signed in before the account had a second factor, as a session that
	// is asked for a step-up is.
	b := signedInBrowser(t, handler, service, marketplace)
	enrolledApp(t, service, marketplace)
	storedEmailFactor(t, marketplace)
	lockCodes(t, marketplace)
	shown := func(language string) string {
		return `<p class="notice" role="status">` + html.EscapeString(codesLocked[language]) + `</p>`
	}
	success := func(language string) string {
		return `<p role="status">` + html.EscapeString(codesLocked[language]) + `</p>`
	}
	alert := func(language string) string {
		return `<p role="alert">` + html.EscapeString(codesLocked[language]) + `</p>`
	}
	notice := func(what, language, body string) {
		t.Helper()
		switch {
		case !strings.Contains(body, shown(language)) || strings.Contains(body, success(language)):
			t.Fatalf("%s in %s: want the lock as a notice, not in the success box: %s", what, language, body)
		case !strings.Contains(body, `[role="status"].notice {`) || !strings.Contains(body, "--notice-bg:"):
			t.Fatalf("%s in %s: the layout gives the notice no colours of its own: %s", what, language, body)
		}
	}
	refusal := func(what, language string, page *httptest.ResponseRecorder) {
		t.Helper()
		body := page.Body.String()
		if page.Code != http.StatusForbidden || !strings.Contains(body, alert(language)) ||
			strings.Contains(body, shown(language)) || strings.Contains(body, success(language)) {
			t.Fatalf("%s in %s: status %d, want %d and the lock as an alert: %s", what, language, page.Code, http.StatusForbidden, body)
		}
	}

	for _, language := range []string{"pt-BR", "en-US"} {
		visitor := passwordDone(t, handler, marketplace, language)
		page := visitor.get("/" + language + "/signin/verify")
		if body := page.Body.String(); page.Code != http.StatusOK || strings.Contains(body, alert(language)) {
			t.Fatalf("the locked second step in %s: status %d, want the lock as a status: %s", language, page.Code, body)
		}
		notice("the locked second step", language, page.Body.String())
		refusal("an app code at the second step", language,
			visitor.post("/"+language+"/signin/verify", url.Values{"method": {"totp"}, "code": {"000000"}}))
		refusal("a code asked for by e-mail at the second step", language,
			visitor.post("/"+language+"/signin/verify/email", url.Values{}))
	}

	for _, language := range []string{"pt-BR", "en-US"} {
		page := b.get("/" + language + "/account/verify?for=add_card&next=%2Faccount%2Fsecurity")
		if body := page.Body.String(); page.Code != http.StatusOK || strings.Contains(body, alert(language)) {
			t.Fatalf("the locked step-up in %s: status %d, want the lock as a status: %s", language, page.Code, body)
		}
		notice("the locked step-up", language, page.Body.String())
		refusal("an app code at a step-up", language, b.post("/"+language+"/account/verify",
			url.Values{"method": {"totp"}, "code": {"000000"}, "for": {"add_card"}, "next": {"/account/security"}}))
		refusal("a code asked for by e-mail at a step-up", language, b.post("/"+language+"/account/verify/email",
			url.Values{"for": {"add_card"}, "next": {"/account/security"}}))
	}
}
