package httpx_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/aleogr/marketplace/internal/platform/httpx"
	"github.com/aleogr/marketplace/internal/platform/i18n"
)

// guarded returns the CSRF middleware, with the pages that write its refusal.
func guarded(t *testing.T) func(http.Handler) http.Handler {
	t.Helper()

	catalogue, err := i18n.Load()
	if err != nil {
		t.Fatalf("i18n.Load() = %v", err)
	}
	return httpx.CSRF(httpx.NewPages(catalogue))
}

// session is one browser: the cookie it holds and the token it was given.
type session struct {
	cookies []*http.Cookie
	token   string
}

// visit performs the GET that any form is rendered by, and keeps what the
// browser would keep.
func visit(t *testing.T) session {
	t.Helper()

	var token string
	handler := guarded(t)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		token = httpx.CSRFToken(r.Context())
	}))

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequestWithContext(
		t.Context(), http.MethodGet, "https://marketplace.example/form", nil))

	response := recorder.Result()
	defer func() { _ = response.Body.Close() }()

	if token == "" {
		t.Fatal("the page was rendered without a token, so no form could send one")
	}
	return session{cookies: response.Cookies(), token: token}
}

// post sends a form as that browser would, with the token given, and returns
// what came back. The response itself does not escape this function, so no
// caller can forget to close it.
func post(t *testing.T, s session, token string, inHeader bool) (int, string) {
	t.Helper()

	handler := guarded(t)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	form := url.Values{}
	if !inHeader {
		form.Set(httpx.CSRFField, token)
	}

	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost,
		"https://marketplace.example/form", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if inHeader {
		request.Header.Set(httpx.CSRFHeader, token)
	}
	for _, cookie := range s.cookies {
		request.AddCookie(cookie)
	}

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	response := recorder.Result()
	defer func() { _ = response.Body.Close() }()

	answer, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("cannot read the response: %v", err)
	}
	return response.StatusCode, string(answer)
}

func TestTheCookieIsLockedDown(t *testing.T) {
	s := visit(t)

	if len(s.cookies) != 1 {
		t.Fatalf("the response set %d cookies, want 1", len(s.cookies))
	}
	cookie := s.cookies[0]

	switch {
	case !cookie.HttpOnly:
		t.Error("the secret is readable from script, which is how it would be copied into a forged request")
	case !cookie.Secure:
		t.Error("the secret may travel over plain HTTP")
	case cookie.SameSite != http.SameSiteLaxMode:
		t.Errorf("SameSite = %v, want Lax", cookie.SameSite)
	case !strings.HasPrefix(cookie.Name, "__Host-"):
		t.Errorf("the cookie is named %q, which a subdomain may overwrite", cookie.Name)
	}
}

func TestAFormWithItsOwnTokenIsAccepted(t *testing.T) {
	s := visit(t)

	for _, inHeader := range []bool{false, true} {
		status, _ := post(t, s, s.token, inHeader)
		if status != http.StatusNoContent {
			t.Errorf("a valid token in header=%v was answered with %d", inHeader, status)
		}
	}
}

func TestARequestWithNoTokenIsRefused(t *testing.T) {
	s := visit(t)

	if status, _ := post(t, s, "", false); status != http.StatusForbidden {
		t.Errorf("a request with no token was answered with %d, want 403", status)
	}
}

// TestATokenFromAnotherSessionIsRefused is the case the delivery names: the
// token is real, and it is not this browser's.
func TestATokenFromAnotherSessionIsRefused(t *testing.T) {
	mine, theirs := visit(t), visit(t)

	if mine.token == theirs.token {
		t.Fatal("two browsers were given the same token")
	}

	if status, _ := post(t, mine, theirs.token, true); status != http.StatusForbidden {
		t.Errorf("another browser's token was answered with %d, want 403", status)
	}
}

// TestATamperedTokenIsRefused covers the signature rather than the cookie: the
// seed is this browser's, the signature is not.
func TestATamperedTokenIsRefused(t *testing.T) {
	s := visit(t)

	seed, _, _ := strings.Cut(s.token, ".")
	tampered := seed + ".AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"

	if status, _ := post(t, s, tampered, true); status != http.StatusForbidden {
		t.Errorf("a tampered token was answered with %d, want 403", status)
	}
}

func TestReadingIsNotGuarded(t *testing.T) {
	handler := guarded(t)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodOptions} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequestWithContext(
			t.Context(), method, "https://marketplace.example/", nil))

		if recorder.Code != http.StatusOK {
			t.Errorf("%s was answered with %d; a safe method changes nothing and needs no token",
				method, recorder.Code)
		}
	}
}

// TestTheRefusalSpeaksBothLanguages: the definition of done is both, and a
// refusal is a page a person reads (docs/requirements.md, section 6).
func TestTheRefusalSpeaksBothLanguages(t *testing.T) {
	s := visit(t)
	_, body := post(t, s, "", false)

	for _, want := range []string{"could not be verified", "Não foi possível verificar"} {
		if !strings.Contains(body, want) {
			t.Errorf("the refusal does not say %q: %s", want, body)
		}
	}
}
