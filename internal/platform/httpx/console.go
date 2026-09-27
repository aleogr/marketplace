package httpx

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/aleogr/marketplace/internal/identity"
	"github.com/aleogr/marketplace/internal/platform/i18n"
	"github.com/aleogr/marketplace/internal/platform/ratelimit"
	"github.com/aleogr/marketplace/internal/platform/version"
	"github.com/aleogr/marketplace/internal/staff"
	"github.com/aleogr/marketplace/internal/tenancy"
	"github.com/aleogr/marketplace/web"
)

// The console lives on a host of its own (F15 spec, D1): the staff's sign-in
// and the console's pages, and nothing of a marketplace. Staff sign in there
// with F13's password and F14's second step, on a platform visit, and must
// have an app or a key before any other page (D4).

// ConsoleVersionPath answers the build identifier, as JSON, to whoever may
// read it (docs/requirements.md, section 27). It is outside the language
// prefixes: it says nothing in any language.
const ConsoleVersionPath = "/console/version"

// enrolPath is where a staff member with no second factor is sent.
const enrolPath = "/enrol"

// setupPath is the first run's page.
const setupPath = "/setup"

// ConsoleRoutes is what the console needs besides the identity flows.
type ConsoleRoutes struct {
	// Authoriser answers whether a staff member may do something. Until the
	// roles arrive it is staff.Owners: the owner may do everything, and
	// nobody else anything (the seam of the F15 plan, PR 1).
	Authoriser staff.Authoriser
	// Bootstrap is the first run, which /setup serves while no owner
	// exists (D2); nil serves none.
	Bootstrap *staff.Bootstrap
	// Setup bounds the first run's attempts per client address, wrong
	// tokens included: the token is long, and this is what makes guessing
	// it hopeless rather than merely unlikely.
	Setup ratelimit.Limiter
}

// WithConsole returns the site serving the console on the console's host. It
// needs the identity routes too, which are how staff sign in.
func (s Site) WithConsole(routes ConsoleRoutes) Site {
	s.console = &routes
	return s
}

// onConsole reports whether a request reached the console's host.
func onConsole(r *http.Request) bool {
	resolution, ok := tenancy.FromContext(r.Context())
	return ok && resolution.Kind == tenancy.ConsoleHost
}

// consoleNameKey carries the console's name into the identity flows: it is
// who the console's mail is from, and what an authenticator app or a key
// names the account's site (visit).
type consoleNameKey struct{}

// consoleRoutes are what the console's host serves. The machinery every host
// has comes first; robots.txt says what it says on a deployment that is not
// indexable, whatever this one's setting, because the console never is.
func (s Site) consoleRoutes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET "+HealthPath, health(s.database))
	mux.HandleFunc("GET "+RobotsPath, robots(false, ""))
	mux.HandleFunc("GET "+ScriptPath, script)
	mux.HandleFunc("POST "+LanguagePath, s.switchLanguage)
	if s.identity == nil || s.console == nil {
		return mux
	}
	id := s.identity
	limit := func(l ratelimit.Limiter, subject ratelimit.Subject, h http.Handler) http.Handler {
		return ratelimit.Limit(l, subject, id.Pages.Refused, id.Log)(h)
	}

	// The first run, while there is no owner (D2).
	mux.HandleFunc("GET "+setupPath, s.setupForm)
	mux.Handle("POST "+setupPath, limit(s.console.Setup, byIP, http.HandlerFunc(s.setup)))

	// Signing in: the password, then F14's second step. Staff answer with an
	// app, a key or a recovery code; the policy refuses them e-mail codes, so
	// the console has no route that sends one.
	mux.HandleFunc("GET /signin", s.signInForm)
	mux.Handle("POST /signin", limit(id.Limits.SignIn, byIP,
		limit(id.Limits.SignInAddress, byAddress, http.HandlerFunc(s.signIn))))
	mux.HandleFunc("GET "+secondStepPath, s.secondStep)
	mux.Handle("POST "+secondStepPath, limit(id.Limits.SignIn, byIP,
		limit(id.Limits.SignInAddress, s.byChallenge, http.HandlerFunc(s.answerSecondStep))))
	mux.HandleFunc("POST /signout", s.signOut)

	// Adding the first app or key is all a staff member with no second factor
	// may do (D4). Every change after the first asks for a step-up, as it
	// does for anybody who has one (F14, D2).
	enrolling := func(back string, h http.HandlerFunc) http.Handler {
		return staffOnly(s.steppedUp(identity.ActionFactors, back, h))
	}
	mux.Handle("GET "+enrolPath, staffOnly(http.HandlerFunc(s.enrolPage)))
	mux.Handle("GET "+securityPath+"/app", enrolling(securityPath+"/app", s.appForm))
	mux.Handle("POST "+securityPath+"/app", enrolling(securityPath+"/app", s.addApp))
	mux.Handle("GET "+securityPath+"/key", enrolling(securityPath+"/key", s.keyForm))
	mux.Handle("POST "+securityPath+"/key", enrolling(securityPath+"/key", s.addKey))

	// Everything else asks for the second factor first.
	secured := func(h http.Handler) http.Handler { return staffOnly(withFactor(h)) }
	changes := func(back string, h http.HandlerFunc) http.Handler {
		return secured(s.steppedUp(identity.ActionFactors, back, h))
	}
	mux.Handle("GET "+securityPath, secured(http.HandlerFunc(s.securityPage)))
	mux.Handle("POST "+securityPath+"/remove", changes(securityPath, s.removeFactor))
	mux.Handle("POST "+securityPath+"/recovery", changes(securityPath, s.regenerateCodes))
	mux.Handle("GET "+stepUpPath, secured(http.HandlerFunc(s.stepUpPage)))
	mux.Handle("POST "+stepUpPath, secured(limit(id.Limits.StepUp, byAccount, http.HandlerFunc(s.answerStepUp))))
	mux.Handle("GET /account/password", secured(
		s.steppedUp(identity.ActionPassword, "/account/password", http.HandlerFunc(s.passwordForm))))
	mux.Handle("POST /account/password", secured(s.steppedUp(identity.ActionPassword, "/account/password",
		limit(id.Limits.Password, byAccount, http.HandlerFunc(s.changePassword)))))

	// The console's own pages, each behind the permission it needs, and
	// "not found" to whoever lacks it (D8).
	mux.Handle("GET /{$}", secured(s.permitted(staff.ConsoleView, http.HandlerFunc(s.consoleHome))))
	mux.Handle("GET "+ConsoleVersionPath, s.consoleVersion())
	return mux
}

// staffOnly serves next only to a signed-in staff member, and sends anybody
// else to sign in. On the console's host the session middleware only ever
// finds a staff session (Sessions), so a session is a staff member's.
func staffOnly(next http.Handler) http.Handler { return signedIn(next) }

// withFactor serves next only to a staff member with a second factor, and
// sends one with none to add it, before any other page (D4).
func withFactor(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if session, _ := SessionFrom(r.Context()); !session.SecondFactor {
			http.Redirect(w, r, "/"+i18n.FromContext(r.Context())+enrolPath, http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// permitted serves next only to a staff member the authoriser allows
// permission, and answers "not found" to anybody else, so that a page does
// not reveal it exists to someone who may not see it (D8).
func (s Site) permitted(permission staff.Permission, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.allows(r, permission) {
			http.NotFound(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// allows reports whether the request's staff member may do permission. A
// question the authoriser cannot answer is a no, and is logged: failing open
// here would show the console to whoever the database was too slow for.
func (s Site) allows(r *http.Request, permission staff.Permission) bool {
	session, ok := SessionFrom(r.Context())
	if !ok || !session.SecondFactor {
		return false
	}
	// Every page of the console so far is the platform's; the pages that
	// show a marketplace's data name it here (F15 spec, D3).
	allowed, err := s.console.Authoriser.Allows(r.Context(), session.Account.ID, permission, staff.Platform)
	if err != nil {
		s.identity.Log.ErrorContext(r.Context(), "the console could not tell what a staff member may do",
			"error", err, "permission", string(permission))
		return false
	}
	return allowed
}

// consoleMenu is the console's navigation for the request: what the signed-in
// staff member may open, and nothing else. A staff member with no second
// factor has no menu: there is one page they may open.
func (s Site) consoleMenu(r *http.Request, path string) []web.MenuItem {
	session, ok := SessionFrom(r.Context())
	if !ok || !session.SecondFactor || s.console == nil {
		return nil
	}
	var menu []web.MenuItem
	for _, entry := range []struct {
		label, path string
		// needs is the permission the entry's page asks for; empty for
		// a staff member's own account, which is theirs whatever they may
		// do elsewhere.
		needs staff.Permission
	}{
		{"console.menu.home", "/", staff.ConsoleView},
		{"console.menu.security", securityPath, ""},
		{"console.menu.password", "/account/password", ""},
	} {
		if entry.needs != "" && !s.allows(r, entry.needs) {
			continue
		}
		menu = append(menu, web.MenuItem{Label: entry.label, Path: entry.path, Current: entry.path == path})
	}
	return menu
}

// enrolPage asks a staff member with no second factor to add an app or a
// key. One who already has one has nothing to do here.
func (s Site) enrolPage(w http.ResponseWriter, r *http.Request) {
	if session, _ := SessionFrom(r.Context()); session.SecondFactor {
		http.Redirect(w, r, "/"+i18n.FromContext(r.Context())+"/", http.StatusSeeOther)
		return
	}
	render(w, r, web.ConsoleEnrol(s.page(r, enrolPath)))
}

// consoleHome is the console's first page.
func (s Site) consoleHome(w http.ResponseWriter, r *http.Request) {
	render(w, r, web.ConsoleHome(s.page(r, "/")))
}

// consoleVersion answers the build identifier as JSON to whoever may read it,
// and "not found" to anybody else, signed in or not (D8). It never redirects
// to sign in: it is read by a person checking a deployment, or by a script.
func (s Site) consoleVersion() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.allows(r, staff.ConsoleVersion) {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(map[string]string{"version": version.String()})
	})
}

// withConsoleName puts the console's name, in the request's language, where
// the identity flows read it (visit).
func (s Site) withConsoleName(r *http.Request) *http.Request {
	name := s.catalogue.Printer(i18n.FromContext(r.Context())).Sprintf("console.title")
	return r.WithContext(context.WithValue(r.Context(), consoleNameKey{}, name))
}

// consoleName is the name withConsoleName put in ctx, or "".
func consoleName(ctx context.Context) string {
	name, _ := ctx.Value(consoleNameKey{}).(string)
	return name
}

// setupOpen reports whether the first run is served, answering "not found"
// when it is not: there is no owner to create any more, or no token to
// create one with (D2, D8).
func (s Site) setupOpen(w http.ResponseWriter, r *http.Request) bool {
	if s.console.Bootstrap == nil {
		http.NotFound(w, r)
		return false
	}
	open, err := s.console.Bootstrap.Open(r.Context())
	if err != nil {
		s.failed(w, r, "the first run could not be looked up", err)
		return false
	}
	if !open {
		http.NotFound(w, r)
	}
	return open
}

// The field each refusal of the first run's form is about.
var setupFields = map[string]string{
	"console.setup.wrong_token":        "token",
	"identity.error.name_missing":      "name",
	"identity.error.name_invalid":      "name",
	"identity.error.email_invalid":     "email",
	"identity.error.password_short":    "password",
	"identity.error.password_long":     "password",
	"identity.error.password_breached": "password",
}

func (s Site) setupForm(w http.ResponseWriter, r *http.Request) {
	if !s.setupOpen(w, r) {
		return
	}
	render(w, r, web.Setup(s.page(r, setupPath), newForm()))
}

// setup creates the owner from the first run's form, signs them in and sends
// them to add a second factor, which the console asks for before anything
// else (D4). A wrong token is told so on its field, as a password the rules
// refuse is on its own; the token itself is never shown back.
func (s Site) setup(w http.ResponseWriter, r *http.Request) {
	if !s.setupOpen(w, r) {
		return
	}
	form := newForm()
	form.Name, form.Email = r.PostFormValue("name"), r.PostFormValue("email")
	token, err := s.console.Bootstrap.Setup(r.Context(), visit(r), r.PostFormValue("token"),
		form.Name, form.Email, r.PostFormValue("password"))
	status := http.StatusUnprocessableEntity
	switch key, args, shown := formError(err); {
	case errors.Is(err, staff.ErrSetupClosed):
		// Another first run got there between the page's check and this
		// one's claim.
		http.NotFound(w, r)
		return
	case errors.Is(err, staff.ErrTokenWrong):
		s.identity.Log.WarnContext(r.Context(), "a first run was refused: the token is wrong")
		form.Error, status = "console.setup.wrong_token", http.StatusUnauthorized
	case shown:
		form.Error, form.ErrorArgs = key, args
	case err != nil:
		s.failed(w, r, "the first run failed", err)
		return
	default:
		s.openSession(w, r, token)
		http.Redirect(w, r, "/"+i18n.FromContext(r.Context())+enrolPath, http.StatusSeeOther)
		return
	}
	form.Field = setupFields[form.Error]
	renderStatus(w, r, status, web.Setup(s.page(r, setupPath), form))
}
