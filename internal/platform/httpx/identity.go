package httpx

import (
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/aleogr/marketplace/internal/identity"
	"github.com/aleogr/marketplace/internal/platform/i18n"
	"github.com/aleogr/marketplace/internal/platform/ratelimit"
	"github.com/aleogr/marketplace/internal/tenancy"
	"github.com/aleogr/marketplace/web"
)

// IdentityLimits are the per-route limits of the identity flows, in the
// database so they hold across instances (docs/roadmap.md, F13). Each limiter
// is named, and namespaces its own counters (internal/platform/ratelimit), so
// the subjects below are just the client, the address or the account.
type IdentityLimits struct {
	SignUp, Resend, ResendAddress, SignIn, SignInAddress, Password ratelimit.Limiter
}

// IdentityRoutes is what the identity pages need.
type IdentityRoutes struct {
	Service *identity.Service
	Limits  IdentityLimits
	Pages   Pages
	Log     *slog.Logger
}

// WithIdentity returns the site serving the identity pages too.
func (s Site) WithIdentity(routes IdentityRoutes) Site {
	s.identity = &routes
	return s
}

// byIP limits by the client's address.
func byIP(r *http.Request) string {
	origin, _ := OriginFrom(r.Context())
	return origin.IP
}

// byAddress limits by the e-mail address a form names, in this marketplace.
func byAddress(r *http.Request) string {
	normalised, _ := identity.NormaliseEmail(r.PostFormValue("email"))
	return marketplaceOf(r) + ":" + normalised
}

func (s Site) identityRoutes(mux *http.ServeMux) {
	if s.identity == nil {
		return
	}
	id := s.identity
	limit := func(l ratelimit.Limiter, subject ratelimit.Subject, h http.Handler) http.Handler {
		return ratelimit.Limit(l, subject, id.Pages.Refused, id.Log)(h)
	}

	mux.Handle("GET /signup", inMarketplace(http.HandlerFunc(s.signUpForm)))
	mux.Handle("POST /signup", inMarketplace(limit(id.Limits.SignUp, byIP, http.HandlerFunc(s.signUp))))
	mux.Handle("GET /verify", inMarketplace(http.HandlerFunc(s.verifyForm)))
	mux.Handle("POST /verify", inMarketplace(http.HandlerFunc(s.verify)))
	mux.Handle("GET /verify/resend", inMarketplace(http.HandlerFunc(s.resendForm)))
	mux.Handle("POST /verify/resend", inMarketplace(limit(id.Limits.Resend, byIP,
		limit(id.Limits.ResendAddress, byAddress, http.HandlerFunc(s.resend)))))
	mux.Handle("GET /signin", inMarketplace(http.HandlerFunc(s.signInForm)))
	mux.Handle("POST /signin", inMarketplace(limit(id.Limits.SignIn, byIP,
		limit(id.Limits.SignInAddress, byAddress, http.HandlerFunc(s.signIn)))))
	mux.Handle("GET "+secondStepPath, inMarketplace(http.HandlerFunc(s.secondStep)))
	mux.Handle("POST "+secondStepPath, inMarketplace(limit(id.Limits.SignIn, byIP,
		limit(id.Limits.SignInAddress, s.byChallenge, http.HandlerFunc(s.answerSecondStep)))))
	mux.Handle("POST /signout", inMarketplace(http.HandlerFunc(s.signOut)))
	mux.Handle("GET /account/password", inMarketplace(signedIn(http.HandlerFunc(s.passwordForm))))
	mux.Handle("POST /account/password", inMarketplace(signedIn(
		limit(id.Limits.Password, byAccount, http.HandlerFunc(s.changePassword)))))
	s.factorRoutes(mux)
}

// signedIn serves next only to a signed-in request, and sends anybody else to
// sign in. It runs before any limit, so a request with no session is not
// counted against an account it does not have.
func signedIn(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := SessionFrom(r.Context()); !ok {
			http.Redirect(w, r, "/"+i18n.FromContext(r.Context())+"/signin", http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// byAccount limits by the signed-in account. It is only mounted behind
// signedIn, so there always is one.
func byAccount(r *http.Request) string {
	session, _ := SessionFrom(r.Context())
	return session.Account.ID
}

// inMarketplace serves next only on a marketplace's host. Accounts belong to
// a marketplace, so the platform's own host has none: there the identity
// routes are not found, before any limit is counted, any password hashed or
// any transaction opened for a marketplace that is not there.
func inMarketplace(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if marketplaceOf(r) == "" {
			http.NotFound(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func marketplaceOf(r *http.Request) string {
	if resolution, ok := tenancy.FromContext(r.Context()); ok && resolution.Marketplace != nil {
		return resolution.Marketplace.ID
	}
	return ""
}

// visit is the request, as the identity flows need it.
func visit(r *http.Request) identity.Visit {
	v := identity.Visit{Language: i18n.FromContext(r.Context()), UserAgent: userAgent(r), BaseURL: linkBase(r)}
	if o, ok := OriginFrom(r.Context()); ok {
		v.IP = o.IP
	}
	if resolution, ok := tenancy.FromContext(r.Context()); ok && resolution.Marketplace != nil {
		v.Marketplace, v.MarketplaceName = resolution.Marketplace.ID, resolution.Marketplace.Name
	}
	return v
}

// maxUserAgent bounds the User-Agent kept with a session or an audit entry.
// Real ones are a few hundred bytes; the header itself may be as large as the
// server accepts, and anybody who knows an address can make a refused sign-in
// store one.
const maxUserAgent = 512

// userAgent is the request's User-Agent as it is stored: valid UTF-8, which is
// all a text column accepts, and at most maxUserAgent bytes, cut between
// characters.
func userAgent(r *http.Request) string {
	agent := strings.ToValidUTF8(r.UserAgent(), "\uFFFD")
	if len(agent) <= maxUserAgent {
		return agent
	}
	cut := maxUserAgent
	for cut > 0 && !utf8.RuneStart(agent[cut]) {
		cut--
	}
	return agent[:cut]
}

// linkBase is the address a mailed link points at: the scheme, the host the
// request was resolved by, normalised as the resolver normalises it, and the
// port only when it is a port. The Host header is the sender's to write, and
// a link built from it verbatim would send somebody else's confirmation
// wherever the sender chose; the host itself is safe because only a
// marketplace's own host reaches these routes, and a port is kept because a
// local run and the end-to-end suite serve on one.
func linkBase(r *http.Request) string {
	host := tenancy.Normalise(r.Host)
	if _, port, err := net.SplitHostPort(r.Host); err == nil && validPort(port) {
		return scheme(r) + "://" + net.JoinHostPort(host, port)
	}
	if strings.Contains(host, ":") {
		// An IPv6 literal with no port still needs its brackets in a URL.
		host = "[" + host + "]"
	}
	return scheme(r) + "://" + host
}

// validPort reports whether port is a TCP port written in decimal digits.
func validPort(port string) bool {
	if port == "" || len(port) > 5 || strings.Trim(port, "0123456789") != "" {
		return false
	}
	n, err := strconv.Atoi(port)
	return err == nil && n >= 1 && n <= 65535
}

// newForm is where every form page starts: the password bounds come from the
// rule itself, so the page, its hint and the rule cannot disagree (spec, D4).
func newForm() web.Form {
	return web.Form{
		MinLength: identity.MinPasswordLength, MaxLength: identity.MaxPasswordLength,
		NameMaxLength: identity.MaxNameLength,
	}
}

// formError names the message a flow's error is shown as, with its
// arguments. An error it does not name is not the visitor's to see.
func formError(err error) (key string, args []any, shown bool) {
	switch {
	case errors.Is(err, identity.ErrNameMissing):
		return "identity.error.name_missing", nil, true
	case errors.Is(err, identity.ErrNameInvalid):
		return "identity.error.name_invalid", []any{identity.MaxNameLength}, true
	case errors.Is(err, identity.ErrEmailInvalid):
		return "identity.error.email_invalid", nil, true
	case errors.Is(err, identity.ErrPasswordShort):
		return "identity.error.password_short", []any{identity.MinPasswordLength}, true
	case errors.Is(err, identity.ErrPasswordLong):
		return "identity.error.password_long", []any{identity.MaxPasswordLength}, true
	case errors.Is(err, identity.ErrPasswordBreached):
		return "identity.error.password_breached", nil, true
	}
	return "", nil, false
}

// The field each refusal is about, by the key it is shown as, form by form:
// that field is marked invalid, points at the refusal and takes the cursor.
// The choice is made here, where the key is known, and not by a template
// reading words. A key a form does not list is about no one field: an
// unconfirmed account is not something the visitor fixes by typing.
var (
	signUpFields = map[string]string{
		"identity.error.name_missing":      "name",
		"identity.error.name_invalid":      "name",
		"identity.error.email_invalid":     "email",
		"identity.error.password_short":    "password",
		"identity.error.password_long":     "password",
		"identity.error.password_breached": "password",
	}
	// One sentence answers an unknown address and a wrong password (D7), and
	// the password is what a visitor who knows their address types again.
	signInFields   = map[string]string{"identity.signin.failed": "password"}
	passwordFields = map[string]string{
		"identity.password.wrong_current":  "current_password",
		"identity.error.password_short":    "new_password",
		"identity.error.password_long":     "new_password",
		"identity.error.password_breached": "new_password",
	}
)

func (s Site) signUpForm(w http.ResponseWriter, r *http.Request) {
	render(w, r, web.SignUp(s.page(r, "/signup"), newForm()))
}

func (s Site) signUp(w http.ResponseWriter, r *http.Request) {
	form := newForm()
	form.Name, form.Email = r.PostFormValue("name"), r.PostFormValue("email")
	err := s.identity.Service.SignUp(r.Context(), visit(r), form.Name, form.Email, r.PostFormValue("password"))
	if key, args, shown := formError(err); shown {
		form.Error, form.ErrorArgs, form.Field = key, args, signUpFields[key]
		renderStatus(w, r, http.StatusUnprocessableEntity, web.SignUp(s.page(r, "/signup"), form))
		return
	}
	if err != nil {
		s.identity.Log.ErrorContext(r.Context(), "sign-up failed", "error", err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	render(w, r, web.CheckEmail(s.page(r, "/signup")))
}

func (s Site) verifyForm(w http.ResponseWriter, r *http.Request) {
	render(w, r, web.Verify(s.page(r, "/verify"), r.URL.Query().Get("token"), false))
}

func (s Site) verify(w http.ResponseWriter, r *http.Request) {
	err := s.identity.Service.Verify(r.Context(), visit(r), r.PostFormValue("token"))
	if errors.Is(err, identity.ErrTokenInvalid) {
		renderStatus(w, r, http.StatusGone, web.Verify(s.page(r, "/verify"), "", true))
		return
	}
	if err != nil {
		s.identity.Log.ErrorContext(r.Context(), "verification failed", "error", err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/"+i18n.FromContext(r.Context())+"/signin?verified=1", http.StatusSeeOther)
}

func (s Site) resendForm(w http.ResponseWriter, r *http.Request) {
	render(w, r, web.Resend(s.page(r, "/verify/resend"), false))
}

func (s Site) resend(w http.ResponseWriter, r *http.Request) {
	if err := s.identity.Service.Resend(r.Context(), visit(r), r.PostFormValue("email")); err != nil {
		s.identity.Log.ErrorContext(r.Context(), "resend failed", "error", err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	render(w, r, web.Resend(s.page(r, "/verify/resend"), true))
}

// signInForm serves the sign-in page; a second step that could no longer be
// answered sends the visitor back here saying why.
func (s Site) signInForm(w http.ResponseWriter, r *http.Request) {
	form := newForm()
	form.Error = again[r.URL.Query().Get("again")]
	render(w, r, web.SignIn(s.page(r, "/signin"), form, r.URL.Query().Get("verified") == "1"))
}

// signIn answers an unknown address and a wrong password with one sentence
// (D7), and tells the owner of an unconfirmed account to confirm it. A
// session opened here revokes the one the browser held and renews the CSRF
// secret. An account with a second factor goes on to the second step, with
// the challenge in its own cookie and no session yet (F14 spec, D3).
func (s Site) signIn(w http.ResponseWriter, r *http.Request) {
	form := newForm()
	form.Email = r.PostFormValue("email")
	token, err := s.identity.Service.SignIn(r.Context(), visit(r), form.Email, r.PostFormValue("password"))
	switch {
	case errors.Is(err, identity.ErrCredentials):
		form.Error = "identity.signin.failed"
	case errors.Is(err, identity.ErrUnverified):
		form.Error = "identity.signin.unverified"
	case errors.Is(err, identity.ErrSecondStep):
		setChallenge(w, r, token)
		http.Redirect(w, r, "/"+i18n.FromContext(r.Context())+secondStepPath, http.StatusSeeOther)
		return
	case err != nil:
		s.identity.Log.ErrorContext(r.Context(), "sign-in failed", "error", err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	default:
		s.openSession(w, r, token)
		http.Redirect(w, r, "/"+i18n.FromContext(r.Context())+"/", http.StatusSeeOther)
		return
	}
	form.Field = signInFields[form.Error]
	renderStatus(w, r, http.StatusUnauthorized, web.SignIn(s.page(r, "/signin"), form, false))
}

// signOut revokes the session the cookie names, if it names one, and clears
// the cookie either way.
func (s Site) signOut(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookieName(r)); err == nil && cookie.Value != "" {
		if err := s.identity.Service.SignOut(r.Context(), visit(r), cookie.Value); err != nil {
			s.identity.Log.ErrorContext(r.Context(), "sign-out failed", "error", err)
		}
	}
	clearSession(w, r)
	http.Redirect(w, r, "/"+i18n.FromContext(r.Context())+"/", http.StatusSeeOther)
}

// passwordForm serves the password page; after a change, which redirects here
// with changed=1, it says the change is done.
func (s Site) passwordForm(w http.ResponseWriter, r *http.Request) {
	render(w, r, web.Password(s.page(r, "/account/password"), newForm(), r.URL.Query().Get("changed") == "1"))
}

// changePassword answers a wrong current password with one sentence, as a
// refused sign-in is answered, and a new password the rule refuses with the
// rule's message. A change ends every session of the account: this browser
// gets the new one's cookie and a new CSRF secret, and is redirected, so a
// reload asks for the page again instead of posting a spent form.
func (s Site) changePassword(w http.ResponseWriter, r *http.Request) {
	session, _ := SessionFrom(r.Context())
	form := newForm()
	token, err := s.identity.Service.ChangePassword(r.Context(), visit(r), session,
		r.PostFormValue("current_password"), r.PostFormValue("new_password"))
	switch key, args, shown := formError(err); {
	case errors.Is(err, identity.ErrCredentials):
		form.Error = "identity.password.wrong_current"
	case shown:
		form.Error, form.ErrorArgs = key, args
	case err != nil:
		s.identity.Log.ErrorContext(r.Context(), "password change failed", "error", err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	default:
		setSession(w, r, token)
		RenewCSRF(w, r)
		http.Redirect(w, r, "/"+i18n.FromContext(r.Context())+"/account/password?changed=1", http.StatusSeeOther)
		return
	}
	form.Field = passwordFields[form.Error]
	renderStatus(w, r, http.StatusUnprocessableEntity, web.Password(s.page(r, "/account/password"), form, false))
}
