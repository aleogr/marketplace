package httpx

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"strings"
)

const (
	// The cookie carries a secret, and the secret never leaves the browser:
	// JavaScript cannot read it, and no other origin can either. The token in
	// the form is derived from it, which is what makes a token minted for one
	// browser useless in another.
	//
	// `__Host-` is a browser-enforced promise — Secure, path `/`, no domain —
	// so a subdomain cannot overwrite the cookie. It requires HTTPS, so a
	// plain local process uses the unprefixed name instead.
	csrfCookie         = "__Host-csrf"
	csrfCookieInsecure = "csrf"

	// CSRFHeader is where HTMX sends the token. Every request it makes carries
	// it through `hx-headers` on the body element, so a form does not have to
	// remember to.
	CSRFHeader = "X-CSRF-Token"
	// CSRFField is where a plain HTML form sends it.
	CSRFField = "csrf_token"

	secretBytes = 32
	tokenBytes  = 16
)

type csrfKey struct{}

// CSRFToken returns the token the current request's page must send back, for
// the templates that render it into a form or onto the body element.
func CSRFToken(ctx context.Context) string {
	token, _ := ctx.Value(csrfKey{}).(string)
	return token
}

// CSRF refuses a state-changing request that does not prove it came from a
// page this deployment served.
//
// The scheme is a signed double submit. The browser holds a secret in a cookie
// it cannot read from script; the page carries a token derived from that
// secret with HMAC-SHA256. Another site can make a browser send the cookie —
// that is what cross-site request forgery is — but it cannot read the secret,
// so it cannot produce a matching token, and a token copied from another
// browser's page was derived from another secret and does not verify here.
//
// `SameSite=Lax` is a second lock on the same door rather than a replacement:
// it is not honoured by every browser in use, and its protection disappears on
// a top-level POST from a site the visitor was already on.
func CSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		secret := csrfSecret(w, r)

		if !safeMethod(r.Method) && !validCSRF(r, secret) {
			forbidden(w)
			return
		}

		token := mintCSRF(secret)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), csrfKey{}, token)))
	})
}

// safeMethod reports whether the method is one that does not change state, and
// therefore needs no token (RFC 9110, section 9.2.1).
func safeMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
		return true
	default:
		return false
	}
}

// csrfSecret returns this browser's secret, setting a new one when it has none
// or when what it sent is not one.
func csrfSecret(w http.ResponseWriter, r *http.Request) []byte {
	name := csrfCookie
	if !isHTTPS(r) {
		name = csrfCookieInsecure
	}

	if cookie, err := r.Cookie(name); err == nil {
		if secret, err := base64.RawURLEncoding.DecodeString(cookie.Value); err == nil && len(secret) == secretBytes {
			return secret
		}
	}

	secret := make([]byte, secretBytes)
	_, _ = rand.Read(secret)

	// Secure is conditional, and that is not a lapse: a deployment is always
	// reached over HTTPS and gets a Secure cookie, while a local process and
	// the end-to-end suite are reached over plain HTTP, where a browser would
	// discard a Secure cookie and nothing could be tested at all.
	// #nosec G124 -- the flags follow the scheme of the request, see above.
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    base64.RawURLEncoding.EncodeToString(secret),
		Path:     "/",
		HttpOnly: true,
		Secure:   isHTTPS(r),
		SameSite: http.SameSiteLaxMode,
	})
	return secret
}

// mintCSRF derives a token from the secret. Each token carries its own random
// half, so two pages of the same session do not share a string an attacker
// could learn once and replay.
func mintCSRF(secret []byte) string {
	seed := make([]byte, tokenBytes)
	_, _ = rand.Read(seed)

	mac := hmac.New(sha256.New, secret)
	mac.Write(seed)

	return base64.RawURLEncoding.EncodeToString(seed) + "." +
		base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// validCSRF reports whether the token the request carries was derived from this
// browser's secret.
func validCSRF(r *http.Request, secret []byte) bool {
	token := r.Header.Get(CSRFHeader)
	if token == "" {
		// Reading the form consumes the body, so it is only read when the
		// header is absent and the body is a form to begin with.
		if contentType := r.Header.Get("Content-Type"); strings.HasPrefix(contentType, "application/x-www-form-urlencoded") ||
			strings.HasPrefix(contentType, "multipart/form-data") {
			if err := r.ParseForm(); err == nil {
				token = r.PostFormValue(CSRFField)
			}
		}
	}

	seed, sum, found := strings.Cut(token, ".")
	if !found {
		return false
	}

	decodedSeed, err := base64.RawURLEncoding.DecodeString(seed)
	if err != nil || len(decodedSeed) != tokenBytes {
		return false
	}
	decodedSum, err := base64.RawURLEncoding.DecodeString(sum)
	if err != nil {
		return false
	}

	mac := hmac.New(sha256.New, secret)
	mac.Write(decodedSeed)
	// Constant time, because a comparison that returns early tells an attacker
	// how much of a guess was right.
	return hmac.Equal(decodedSum, mac.Sum(nil))
}
