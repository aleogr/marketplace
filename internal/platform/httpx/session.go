package httpx

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/aleogr/marketplace/internal/identity"
)

// The session cookie follows the CSRF cookie's rules (csrf.go): `__Host-` over
// HTTPS, bound to this marketplace's host, never readable by script.
const (
	sessionCookie         = "__Host-session"
	sessionCookieInsecure = "session"
)

type sessionKey struct{}

// SessionFrom returns the signed-in session of a request, if there is one.
func SessionFrom(ctx context.Context) (identity.Session, bool) {
	s, ok := ctx.Value(sessionKey{}).(identity.Session)
	return s, ok
}

func sessionCookieName(r *http.Request) string {
	if isHTTPS(r) {
		return sessionCookie
	}
	return sessionCookieInsecure
}

// Sessions puts the signed-in account in the request's context. A cookie that
// no longer opens a session is cleared, so the browser stops sending it. A
// request with no cookie, or on a host that is no marketplace, never reaches
// the database.
func Sessions(service *identity.Service, log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(sessionCookieName(r))
			marketplace := marketplaceOf(r)
			if err != nil || cookie.Value == "" || marketplace == "" {
				next.ServeHTTP(w, r)
				return
			}
			session, err := service.Authenticate(r.Context(), marketplace, cookie.Value)
			switch {
			case errors.Is(err, identity.ErrSessionInvalid):
				clearSession(w, r)
			case err != nil:
				log.ErrorContext(r.Context(), "a session could not be checked", "error", err)
			default:
				r = r.WithContext(context.WithValue(r.Context(), sessionKey{}, session))
			}
			next.ServeHTTP(w, r)
		})
	}
}

func setSession(w http.ResponseWriter, r *http.Request, token string) {
	// #nosec G124 -- Secure follows the scheme, as the CSRF cookie's does (csrf.go).
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookieName(r), Value: token, Path: "/", HttpOnly: true,
		Secure: isHTTPS(r), SameSite: http.SameSiteLaxMode,
		MaxAge: int(identity.SessionLifetime.Seconds()),
	})
}

func clearSession(w http.ResponseWriter, r *http.Request) {
	// #nosec G124 -- see setSession.
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookieName(r), Value: "", Path: "/", HttpOnly: true,
		Secure: isHTTPS(r), SameSite: http.SameSiteLaxMode, MaxAge: -1,
	})
}
