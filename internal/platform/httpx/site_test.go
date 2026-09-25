package httpx_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/aleogr/marketplace/internal/platform/httpx"
	"github.com/aleogr/marketplace/internal/platform/i18n"
)

// switchTo posts the language switch as a page's form would, and returns where
// the visitor was sent and what was remembered.
func switchTo(t *testing.T, language, path string) (location string, cookie *http.Cookie) {
	t.Helper()

	form := url.Values{"language": {language}, "path": {path}}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost,
		"https://marketplace1.example"+httpx.LanguagePath, strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	recorder := httptest.NewRecorder()
	routes(t, nil).ServeHTTP(recorder, request)

	for _, set := range recorder.Result().Cookies() {
		if set.Name == i18n.Cookie {
			cookie = set
		}
	}
	return recorder.Header().Get("Location"), cookie
}

func TestTheSwitchRemembersTheChoiceAndStaysOnThePage(t *testing.T) {
	location, cookie := switchTo(t, "pt-BR", "/offers/17")

	if location != "/pt-BR/offers/17" {
		t.Errorf("Location = %q, want the same page in the language chosen", location)
	}
	if cookie == nil {
		t.Fatal("nothing was remembered, so the next visit would guess again")
	}
	if cookie.Value != "pt-BR" {
		t.Errorf("the cookie remembers %q", cookie.Value)
	}
	// A year: the choice is a preference, not a session (docs/design.md,
	// decision 10).
	if cookie.MaxAge < 300*24*60*60 {
		t.Errorf("the choice is remembered for %d seconds, which is not a year", cookie.MaxAge)
	}
}

// TestTheSwitchCannotBeTurnedIntoALinkToSomewhereElse is the reason the form's
// path is not used as given.
//
// The path comes from a form, so it is a visitor's to write, and a redirect
// that took a whole address from one would be an open redirect: a link with
// this deployment's name on it that lands on somebody else's login page.
func TestTheSwitchCannotBeTurnedIntoALinkToSomewhereElse(t *testing.T) {
	for _, path := range []string{
		"//evil.example/login",
		"https://evil.example/login",
		"/\\evil.example",
		"/offers\r\nLocation: https://evil.example",
	} {
		location, _ := switchTo(t, "pt-BR", path)

		if !strings.HasPrefix(location, "/pt-BR/") {
			t.Errorf("a path of %q sent the visitor to %q", path, location)
		}
		if strings.Contains(location, "evil.example") {
			t.Errorf("a path of %q reached another site: %q", path, location)
		}
	}
}

func TestAnUnknownLanguageIsNotRemembered(t *testing.T) {
	location, cookie := switchTo(t, "fr-FR", "/")

	if cookie != nil && cookie.Value == "fr-FR" {
		t.Error("a language the platform does not speak was remembered")
	}
	if !strings.HasPrefix(location, "/"+i18n.Default) {
		t.Errorf("Location = %q, want the official language", location)
	}
}

// switchWithQuery posts the language switch from a page whose address carried
// query, and returns where the visitor was sent.
func switchWithQuery(t *testing.T, path, query string) string {
	t.Helper()

	form := url.Values{"language": {"pt-BR"}, "path": {path}, "query": {query}}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost,
		"https://marketplace1.example"+httpx.LanguagePath, strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	recorder := httptest.NewRecorder()
	routes(t, nil).ServeHTTP(recorder, request)
	return recorder.Header().Get("Location")
}

// A page whose address carries a query — the step-up's action and return
// path, the second step's method — is still that page after the switch.
func TestTheSwitchKeepsThePagesQuery(t *testing.T) {
	location := switchWithQuery(t, "/account/verify", "for=password&next=%2Faccount%2Fpassword")

	if location != "/pt-BR/account/verify?for=password&next=%2Faccount%2Fpassword" {
		t.Errorf("Location = %q, want the same page with its query, in the language chosen", location)
	}
}

// The query comes from a form too, so it is a visitor's to write: it is read
// as a query and written back as one, and whatever it holds stays in the
// query. It can neither add a header nor change the host or the path.
func TestTheSwitchsQueryCannotLeadAnywhereElse(t *testing.T) {
	for _, query := range []string{
		"x\r\nLocation: //evil.example",
		"//evil.example",
		"x=1#@evil.example/login",
		"x=1&/../../evil",
		"%zz",
	} {
		location := switchWithQuery(t, "/offers/17", query)

		if strings.ContainsAny(location, "\r\n#") {
			t.Errorf("a query of %q wrote %q into the Location", query, location)
		}
		target, err := url.Parse(location)
		if err != nil {
			t.Fatalf("a query of %q gave the Location %q, which does not parse: %v", query, location, err)
		}
		if target.Scheme != "" || target.Host != "" || target.Path != "/pt-BR/offers/17" {
			t.Errorf("a query of %q sent the visitor to %q", query, location)
		}
	}
}
