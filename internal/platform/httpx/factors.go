package httpx

import (
	"encoding/base64"
	"errors"
	"net/http"
	"strings"

	"rsc.io/qr"

	"github.com/aleogr/marketplace/internal/identity"
	"github.com/aleogr/marketplace/internal/platform/i18n"
	"github.com/aleogr/marketplace/web"
)

// The security page and the pages that add and remove second factors
// (docs/superpowers/specs/2026-09-25-f14-two-factor-design.md). Every one is
// served only signed in, and only on a marketplace's host.
func (s Site) factorRoutes(mux *http.ServeMux) {
	// Every change needs a recent step-up once the account has 2FA (D2, D4);
	// reading the page does not.
	changes := func(back string, h http.HandlerFunc) http.Handler {
		return inMarketplace(signedIn(s.steppedUp(identity.ActionFactors, back, h)))
	}
	mux.Handle("GET /account/security", inMarketplace(signedIn(http.HandlerFunc(s.securityPage))))
	mux.Handle("GET /account/security/app", changes(securityPath+"/app", s.appForm))
	mux.Handle("POST /account/security/app", changes(securityPath+"/app", s.addApp))
	mux.Handle("POST /account/security/remove", changes(securityPath, s.removeFactor))
	mux.Handle("POST /account/security/recovery", changes(securityPath, s.regenerateCodes))
}

// securityPath is where the security pages live, below the language.
const securityPath = "/account/security"

// securityDone is what a redirect back to the security page may say was
// done: the query names it, and only these keys are shown.
var securityDone = map[string]string{
	"added":   "identity.security.done.added",
	"removed": "identity.security.done.removed",
}

// toSecurity sends the visitor back to the security page, saying what was
// done.
func toSecurity(w http.ResponseWriter, r *http.Request, done string) {
	http.Redirect(w, r, "/"+i18n.FromContext(r.Context())+securityPath+"?done="+done, http.StatusSeeOther)
}

// failed logs an error the visitor cannot act on and answers 500.
func (s Site) failed(w http.ResponseWriter, r *http.Request, what string, err error) {
	s.identity.Log.ErrorContext(r.Context(), what, "error", err)
	http.Error(w, "", http.StatusInternalServerError)
}

// security is the security page's content for the signed-in account.
func (s Site) security(r *http.Request) (web.Security, error) {
	session, _ := SessionFrom(r.Context())
	found, err := s.identity.Service.Security(r.Context(), visit(r), session)
	if err != nil {
		return web.Security{}, err
	}
	tag := i18n.FromContext(r.Context())
	view := web.Security{RecoveryIssued: found.RecoveryIssued, RecoveryLeft: found.RecoveryLeft}
	for _, f := range found.Factors {
		row := web.Factor{ID: f.ID, Method: string(f.Method), Label: f.Label,
			Added: s.catalogue.Date(tag, f.CreatedAt, "format.date.short")}
		if f.LastUsedAt != nil {
			row.LastUsed = s.catalogue.Date(tag, *f.LastUsedAt, "format.date.short")
		}
		view.Factors = append(view.Factors, row)
	}
	return view, nil
}

func (s Site) securityPage(w http.ResponseWriter, r *http.Request) {
	view, err := s.security(r)
	if err != nil {
		s.failed(w, r, "the security page could not be read", err)
		return
	}
	done := r.URL.Query().Get("done")
	view.Done = securityDone[done]
	if done == "recovery_used" {
		// Signed in with a recovery code: say how many are left, which is
		// the moment to make new ones.
		view.Done, view.DoneArgs = "identity.security.done.recovery_used", []any{view.RecoveryLeft}
	}
	render(w, r, web.SecurityPage(s.page(r, securityPath), view))
}

// qrDataURI draws text as a QR code and returns it as a PNG data URI, which
// the Content Security Policy's img-src already allows: the secret it
// carries never leaves this response for another server to draw.
func qrDataURI(text string) (string, error) {
	code, err := qr.Encode(text, qr.M)
	if err != nil {
		return "", err
	}
	code.Scale = 5
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(code.PNG()), nil
}

// grouped writes a key in groups of four, which is how a person copies it.
func grouped(key string) string {
	var out strings.Builder
	for i, r := range key {
		if i > 0 && i%4 == 0 {
			out.WriteByte(' ')
		}
		out.WriteRune(r)
	}
	return out.String()
}

// renderApp shows the enrolment of an app, with a refusal when there is one.
func (s Site) renderApp(w http.ResponseWriter, r *http.Request, status int, enrolment identity.AppEnrolment, view web.AppEnrolment) {
	uri, err := qrDataURI(enrolment.URI)
	if err != nil {
		s.failed(w, r, "the QR code could not be drawn", err)
		return
	}
	view.QR, view.Key, view.LabelMax = uri, grouped(enrolment.Key), identity.MaxLabelLength
	renderStatus(w, r, status, web.EnrolApp(s.page(r, securityPath+"/app"), view))
}

// appForm starts adding an app: each visit draws a new secret, so a page
// left open elsewhere is not the one that is confirmed.
func (s Site) appForm(w http.ResponseWriter, r *http.Request) {
	session, _ := SessionFrom(r.Context())
	enrolment, err := s.identity.Service.BeginApp(r.Context(), visit(r), session)
	if errors.Is(err, identity.ErrNotPermitted) {
		http.NotFound(w, r)
		return
	}
	if errors.Is(err, identity.ErrStepUpNeeded) {
		toStepUp(w, r, identity.ActionFactors, securityPath+"/app")
		return
	}
	if err != nil {
		s.failed(w, r, "an app enrolment could not start", err)
		return
	}
	s.renderApp(w, r, http.StatusOK, enrolment, web.AppEnrolment{})
}

// The field each refusal of the app's form is about.
var appFields = map[string]string{
	"identity.app.wrong_code":      "code",
	"identity.error.label_invalid": "label",
}

// addApp confirms the app with a code it made. The first app or key shows
// the recovery codes, once; a later one goes back to the security page. A
// wrong code shows the same QR code again, so the app already set up still
// works.
func (s Site) addApp(w http.ResponseWriter, r *http.Request) {
	session, _ := SessionFrom(r.Context())
	label := r.PostFormValue("label")
	codes, err := s.identity.Service.ConfirmApp(r.Context(), visit(r), session, label, r.PostFormValue("code"))
	view := web.AppEnrolment{Label: label}
	switch {
	case errors.Is(err, identity.ErrNoEnrolment):
		http.Redirect(w, r, "/"+i18n.FromContext(r.Context())+securityPath+"/app", http.StatusSeeOther)
		return
	case errors.Is(err, identity.ErrNotPermitted):
		http.NotFound(w, r)
		return
	case errors.Is(err, identity.ErrStepUpNeeded):
		toStepUp(w, r, identity.ActionFactors, securityPath+"/app")
		return
	case errors.Is(err, identity.ErrCodeWrong):
		view.Form.Error = "identity.app.wrong_code"
	case errors.Is(err, identity.ErrLabelInvalid):
		view.Form.Error, view.Form.ErrorArgs = "identity.error.label_invalid", []any{identity.MaxLabelLength}
	case err != nil:
		s.failed(w, r, "an app could not be added", err)
		return
	case codes == nil:
		toSecurity(w, r, "added")
		return
	default:
		render(w, r, web.RecoveryCodes(s.page(r, securityPath), web.Recovery{Codes: codes, Added: true}))
		return
	}
	view.Form.Field = appFields[view.Form.Error]
	enrolment, err := s.identity.Service.PendingApp(r.Context(), visit(r), session)
	if errors.Is(err, identity.ErrNoEnrolment) {
		http.Redirect(w, r, "/"+i18n.FromContext(r.Context())+securityPath+"/app", http.StatusSeeOther)
		return
	}
	if err != nil {
		s.failed(w, r, "an app enrolment could not be read", err)
		return
	}
	s.renderApp(w, r, http.StatusUnprocessableEntity, enrolment, view)
}

// removeFactor removes one of the account's second factors. One the account
// does not have is not found, whoever's it is.
func (s Site) removeFactor(w http.ResponseWriter, r *http.Request) {
	session, _ := SessionFrom(r.Context())
	err := s.identity.Service.RemoveFactor(r.Context(), visit(r), session, r.PostFormValue("factor"))
	switch {
	case errors.Is(err, identity.ErrStepUpNeeded):
		toStepUp(w, r, identity.ActionFactors, securityPath)
	case errors.Is(err, identity.ErrFactorUnknown):
		http.NotFound(w, r)
	case errors.Is(err, identity.ErrFactorRequired):
		view, err := s.security(r)
		if err != nil {
			s.failed(w, r, "the security page could not be read", err)
			return
		}
		view.Form.Error = "identity.security.error.required"
		renderStatus(w, r, http.StatusUnprocessableEntity, web.SecurityPage(s.page(r, securityPath), view))
	case err != nil:
		s.failed(w, r, "a second factor could not be removed", err)
	default:
		toSecurity(w, r, "removed")
	}
}

// regenerateCodes replaces the recovery codes and shows the new ones, once.
func (s Site) regenerateCodes(w http.ResponseWriter, r *http.Request) {
	session, _ := SessionFrom(r.Context())
	codes, err := s.identity.Service.RegenerateRecoveryCodes(r.Context(), visit(r), session)
	switch {
	case errors.Is(err, identity.ErrStepUpNeeded):
		toStepUp(w, r, identity.ActionFactors, securityPath)
	case errors.Is(err, identity.ErrNoSecondFactor):
		http.Redirect(w, r, "/"+i18n.FromContext(r.Context())+securityPath, http.StatusSeeOther)
	case err != nil:
		s.failed(w, r, "the recovery codes could not be regenerated", err)
	default:
		render(w, r, web.RecoveryCodes(s.page(r, securityPath), web.Recovery{Codes: codes}))
	}
}
