package mail_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/aleogr/marketplace/internal/platform/i18n"
	"github.com/aleogr/marketplace/internal/platform/mail"
)

// templates returns what the binary carries.
func templates(t *testing.T) *mail.Templates {
	t.Helper()

	loaded, err := mail.LoadTemplates()
	if err != nil {
		t.Fatalf("mail.LoadTemplates() = %v", err)
	}
	return loaded
}

// sampleVariables fills every variable any template asks for. The templates
// are parsed with missingkey=error, so a template that asks for a variable
// missing here fails this test: add the variable here when a template gains
// one.
var sampleVariables = map[string]string{
	"Name":    "Reader",
	"Link":    "https://marketplace1.example/en-US/verify?token=sample",
	"SignIn":  "https://marketplace1.example/en-US/signin",
	"Code":    "123456",
	"Method":  "totp",
	"Left":    "9",
	"Purpose": "signin",
}

// The definition of done: every user-facing text exists in both languages
// (CLAUDE.md). A template is user-facing text, and a reader who chose pt-BR
// and receives nothing is the failure this prevents.
func TestEveryTemplateExistsInEveryLanguageThePlatformSpeaks(t *testing.T) {
	catalogue, err := i18n.Load()
	if err != nil {
		t.Fatalf("i18n.Load() = %v", err)
	}

	loaded := templates(t)
	for _, language := range catalogue.Languages() {
		for _, name := range loaded.Names() {
			rendered, err := loaded.Render(mail.Message{
				Template: name, Language: language, To: "reader@example.test",
				From: "Marketplace 1", Variables: sampleVariables,
			}, i18n.Default)
			if err != nil {
				t.Fatalf("the template %q does not render in %s: %v", name, language, err)
			}
			if rendered.Language != language {
				t.Errorf("%q in %s fell back to %s, so that language has no template of its own",
					name, language, rendered.Language)
			}
		}
	}
}

func TestATemplateCarriesItsSubjectAndBothParts(t *testing.T) {
	rendered, err := templates(t).Render(mail.Message{
		Template: "probe", Language: "pt-BR", To: "reader@example.test",
		From: "Marketplace 1",
	}, i18n.Default)
	if err != nil {
		t.Fatalf("Render() = %v", err)
	}

	switch {
	case rendered.Subject != "Teste de entrega de e-mail — Marketplace 1":
		t.Errorf("subject = %q", rendered.Subject)
	case strings.HasPrefix(rendered.Text, "Subject:"):
		t.Errorf("the subject line was left in the body: %q", rendered.Text)
	case !strings.Contains(rendered.Text, "Marketplace 1"):
		t.Errorf("the text part did not receive the sender's name: %q", rendered.Text)
	case !strings.Contains(rendered.HTML, "<p>"):
		t.Errorf("the HTML part is not HTML: %q", rendered.HTML)
	}
}

// A language with no template of its own is answered in the official one. A
// message in English reaches its reader; a message that failed to render does
// not (docs/requirements.md, section 6).
func TestALanguageWithNoTemplateFallsBackToTheOfficialOne(t *testing.T) {
	rendered, err := templates(t).Render(mail.Message{
		Template: "probe", Language: "fr-FR", To: "reader@example.test",
		From: "Marketplace 1",
	}, i18n.Default)
	if err != nil {
		t.Fatalf("Render() = %v", err)
	}

	if rendered.Language != i18n.Default {
		t.Errorf("language = %q, want %q", rendered.Language, i18n.Default)
	}
	if !strings.Contains(rendered.Text, "can deliver e-mail") {
		t.Errorf("the message did not come out in the official language: %q", rendered.Text)
	}
}

func TestATemplateThatDoesNotExistIsAnError(t *testing.T) {
	_, err := templates(t).Render(mail.Message{
		Template: "no-such-template", Language: "en-US", To: "reader@example.test",
	}, i18n.Default)

	if !errors.Is(err, mail.ErrNoTemplate) {
		t.Errorf("Render() = %v, want %v", err, mail.ErrNoTemplate)
	}
}

func TestAnAddressIsComparedInOneForm(t *testing.T) {
	for _, given := range []string{"Reader@Example.Test", "  reader@example.test  ", "READER@EXAMPLE.TEST"} {
		if got := mail.Address(given); got != "reader@example.test" {
			t.Errorf("Address(%q) = %q", given, got)
		}
	}
}

// The second-factor notices name the method in the reader's language: the
// service passes its kind, and each template words it.
func TestTheSecondFactorNoticesNameTheMethodInTheReadersLanguage(t *testing.T) {
	for _, tc := range []struct{ template, language, method, want string }{
		{"second-factor-added", "pt-BR", "webauthn", "uma chave de segurança ou dispositivo"},
		{"second-factor-added", "en-US", "totp", "an authenticator app"},
		{"second-factor-removed", "en-US", "email", "codes by e-mail"},
		{"second-factor-removed", "pt-BR", "totp", "um aplicativo autenticador"},
	} {
		rendered, err := templates(t).Render(mail.Message{
			Template: tc.template, Language: tc.language, To: "reader@example.test", From: "Loja Um",
			Variables: map[string]string{"Name": "Leitora", "Method": tc.method},
		}, i18n.Default)
		if err != nil {
			t.Fatalf("%s in %s: %v", tc.template, tc.language, err)
		}
		if !strings.Contains(rendered.Text, tc.want) || !strings.Contains(rendered.HTML, tc.want) {
			t.Errorf("%s in %s with %s does not say %q: %s", tc.template, tc.language, tc.method, tc.want, rendered.Text)
		}
	}
}

// The notices of a run of failed second factors exist in both languages and
// both parts, name the reader and the marketplace, and tell the reader to
// change the password (F14 spec, D8). A failed step-up counts as a failed
// second step does, so they do not assume a sign-in: whoever failed knows the
// password or is signed in to the account.
func TestTheFailureNoticesExistInBothLanguages(t *testing.T) {
	for _, tc := range []struct{ template, language, want, who, never string }{
		{"second-factor-failures", "en-US", "change your password now", "or is signed in to your account", "second step of signing in"},
		{"second-factor-failures", "pt-BR", "altere sua senha agora", "ou que entrou na sua conta", "segunda etapa da entrada"},
		{"second-factor-locked", "en-US", "Change your password now", "or is signed in to your account", "second step of signing in"},
		{"second-factor-locked", "pt-BR", "Altere sua senha agora", "ou que entrou na sua conta", "segunda etapa da entrada"},
	} {
		rendered, err := templates(t).Render(mail.Message{
			Template: tc.template, Language: tc.language, To: "reader@example.test", From: "Loja Um",
			Variables: map[string]string{"Name": "Leitora"},
		}, i18n.Default)
		if err != nil {
			t.Fatalf("%s in %s: %v", tc.template, tc.language, err)
		}
		if rendered.Language != tc.language || !strings.Contains(rendered.Subject, "Loja Um") {
			t.Errorf("%s in %s: language %s, subject %q", tc.template, tc.language, rendered.Language, rendered.Subject)
		}
		for _, part := range []string{rendered.Text, rendered.HTML} {
			if !strings.Contains(part, "Leitora") || !strings.Contains(part, "Loja Um") || !strings.Contains(part, tc.want) {
				t.Errorf("%s in %s does not name the reader and the marketplace and say %q: %s", tc.template, tc.language, tc.want, part)
			}
			if !strings.Contains(part, tc.who) || strings.Contains(part, tc.never) {
				t.Errorf("%s in %s does not say %q, or assumes a sign-in with %q: %s", tc.template, tc.language, tc.who, tc.never, part)
			}
		}
	}
}

// The code's mail says what the code is for — signing in, confirming a
// change, adding e-mail as a second factor — in the reader's language, and
// keeps the code out of the subject, which lock screens and the provider's
// activity logs show; the body carries it.
func TestTheCodeMailSaysWhatItIsForAndKeepsTheCodeOutOfTheSubject(t *testing.T) {
	for _, tc := range []struct{ language, purpose, want string }{
		{"en-US", "signin", "to sign in"},
		{"en-US", "stepup", "to confirm a change"},
		{"en-US", "enrol", "to add e-mail as a second factor"},
		{"pt-BR", "signin", "para entrar"},
		{"pt-BR", "stepup", "para confirmar uma alteração"},
		{"pt-BR", "enrol", "para adicionar o e-mail como segundo fator"},
	} {
		rendered, err := templates(t).Render(mail.Message{
			Template: "second-factor-code", Language: tc.language, To: "reader@example.test", From: "Loja Um",
			Variables: map[string]string{"Name": "Leitora", "Code": "481516", "Purpose": tc.purpose},
		}, i18n.Default)
		if err != nil {
			t.Fatalf("second-factor-code in %s for %s: %v", tc.language, tc.purpose, err)
		}
		if strings.Contains(rendered.Subject, "481516") {
			t.Errorf("the subject in %s for %s carries the code: %q", tc.language, tc.purpose, rendered.Subject)
		}
		if !strings.Contains(rendered.Subject, "Loja Um") || !strings.Contains(rendered.Subject, tc.want) {
			t.Errorf("the subject in %s for %s = %q; want the marketplace and %q", tc.language, tc.purpose, rendered.Subject, tc.want)
		}
		for _, part := range []string{rendered.Text, rendered.HTML} {
			if !strings.Contains(part, "481516") || !strings.Contains(part, tc.want) {
				t.Errorf("second-factor-code in %s for %s does not carry the code and say %q: %s", tc.language, tc.purpose, tc.want, part)
			}
		}
	}
}

// The recovery code's notice says what the code was used for: a recovery code
// answers a step-up as well as the second step of signing in, so the notice
// of one used to confirm a change does not claim that someone signed in.
func TestTheRecoveryNoticeSaysWhatTheCodeWasUsedFor(t *testing.T) {
	for _, tc := range []struct {
		language, purpose, want string
		never                   []string
	}{
		{"en-US", "signin", "to sign in", []string{"to confirm a change"}},
		{"en-US", "stepup", "to confirm a change", []string{"to sign in", "signed in with"}},
		{"pt-BR", "signin", "para entrar", []string{"para confirmar uma alteração"}},
		{"pt-BR", "stepup", "para confirmar uma alteração", []string{"para entrar", "acabou de entrar"}},
	} {
		rendered, err := templates(t).Render(mail.Message{
			Template: "recovery-code-used", Language: tc.language, To: "reader@example.test", From: "Loja Um",
			Variables: map[string]string{"Name": "Leitora", "Left": "7", "Purpose": tc.purpose},
		}, i18n.Default)
		if err != nil {
			t.Fatalf("recovery-code-used in %s for %s: %v", tc.language, tc.purpose, err)
		}
		if rendered.Language != tc.language || !strings.Contains(rendered.Subject, "Loja Um") ||
			!strings.Contains(rendered.Subject, tc.want) {
			t.Errorf("the subject in %s for %s = %q (%s); want the marketplace and %q",
				tc.language, tc.purpose, rendered.Subject, rendered.Language, tc.want)
		}
		for _, part := range []string{rendered.Subject, rendered.Text, rendered.HTML} {
			for _, never := range tc.never {
				if strings.Contains(part, never) {
					t.Errorf("recovery-code-used in %s for %s says %q: %s", tc.language, tc.purpose, never, part)
				}
			}
		}
		for _, part := range []string{rendered.Text, rendered.HTML} {
			if !strings.Contains(part, "Leitora") || !strings.Contains(part, "7") || !strings.Contains(part, tc.want) {
				t.Errorf("recovery-code-used in %s for %s does not name the reader and the codes left and say %q: %s",
					tc.language, tc.purpose, tc.want, part)
			}
		}
	}
}
