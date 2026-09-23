package httpx

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/aleogr/marketplace/internal/identity"
	"github.com/aleogr/marketplace/internal/platform/i18n"
	"github.com/aleogr/marketplace/internal/platform/ratelimit"
	"github.com/aleogr/marketplace/internal/tenancy"
	"github.com/aleogr/marketplace/web"
)

// IdentityLimits are the per-route limits of the identity flows, in the
// database so they hold across instances (docs/roadmap.md, F13). Each limiter
// is named, and namespaces its own counters (internal/platform/ratelimit), so
// the subjects below are just the client or the address.
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

	mux.HandleFunc("GET /signup", s.signUpForm)
	mux.Handle("POST /signup", limit(id.Limits.SignUp, byIP, http.HandlerFunc(s.signUp)))
	mux.HandleFunc("GET /verify", s.verifyForm)
	mux.HandleFunc("POST /verify", s.verify)
	mux.HandleFunc("GET /verify/resend", s.resendForm)
	mux.Handle("POST /verify/resend", limit(id.Limits.Resend, byIP,
		limit(id.Limits.ResendAddress, byAddress, http.HandlerFunc(s.resend))))
}

func marketplaceOf(r *http.Request) string {
	if resolution, ok := tenancy.FromContext(r.Context()); ok && resolution.Marketplace != nil {
		return resolution.Marketplace.ID
	}
	return ""
}

// visit is the request, as the identity flows need it.
func visit(r *http.Request) identity.Visit {
	v := identity.Visit{Language: i18n.FromContext(r.Context()), UserAgent: r.UserAgent(), BaseURL: origin(r)}
	if o, ok := OriginFrom(r.Context()); ok {
		v.IP = o.IP
	}
	if resolution, ok := tenancy.FromContext(r.Context()); ok && resolution.Marketplace != nil {
		v.Marketplace, v.MarketplaceName = resolution.Marketplace.ID, resolution.Marketplace.Name
	}
	return v
}

// newForm is where every form page starts: the password bounds come from the
// rule itself, so the page, its hint and the rule cannot disagree (spec, D4).
func newForm() web.Form {
	return web.Form{MinLength: identity.MinPasswordLength, MaxLength: identity.MaxPasswordLength}
}

// formError names the message a flow's error is shown as, with its
// arguments. An error it does not name is not the visitor's to see.
func formError(err error) (key string, args []any, shown bool) {
	switch {
	case errors.Is(err, identity.ErrNameMissing):
		return "identity.error.name_missing", nil, true
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

func (s Site) signUpForm(w http.ResponseWriter, r *http.Request) {
	render(w, r, web.SignUp(s.page(r, "/signup"), newForm()))
}

func (s Site) signUp(w http.ResponseWriter, r *http.Request) {
	form := newForm()
	form.Name, form.Email = r.PostFormValue("name"), r.PostFormValue("email")
	err := s.identity.Service.SignUp(r.Context(), visit(r), form.Name, form.Email, r.PostFormValue("password"))
	if key, args, shown := formError(err); shown {
		form.Error, form.ErrorArgs = key, args
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
