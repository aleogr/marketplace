package httpx

import (
	"errors"
	"net/http"
	"net/url"

	"github.com/aleogr/marketplace/internal/identity"
	"github.com/aleogr/marketplace/internal/platform/i18n"
	"github.com/aleogr/marketplace/web"
)

// stepUpPath is the step-up page: the same challenge page as the second step
// of a sign-in, for a session that must prove its second factor again before
// a sensitive action (F14 spec, D2, D4).
const stepUpPath = "/account/verify"

// toStepUp sends the visitor to prove a second factor before action, and to
// come back to back afterwards.
func toStepUp(w http.ResponseWriter, r *http.Request, action identity.Action, back string) {
	query := url.Values{"for": {string(action)}, "next": {back}}
	http.Redirect(w, r, "/"+i18n.FromContext(r.Context())+stepUpPath+"?"+query.Encode(), http.StatusSeeOther)
}

// steppedUp serves next only to a session that needs no step-up for action,
// and sends any other to the step-up page, to come back to back. It is
// mounted behind signedIn, so there always is a session.
func (s Site) steppedUp(action identity.Action, back string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, _ := SessionFrom(r.Context())
		if s.identity.Service.NeedsStepUp(session, action) {
			toStepUp(w, r, action, back)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// stepUpAgain is what the step-up page may say when a challenge could no
// longer be answered and a new one was opened.
var stepUpAgain = map[string]string{
	"expired":   "identity.stepup.expired",
	"exhausted": "identity.stepup.exhausted",
}

// stepUpTarget reads what a step-up is for and where it returns to: an
// action the policy names, and a path on this site, never another's.
func stepUpTarget(action, next string) (identity.Action, string, bool) {
	parsed, ok := identity.ParseAction(action)
	return parsed, safePath(next), ok
}

// stepUpBase is the step-up page's own address, for the links to its other
// methods.
func stepUpBase(r *http.Request, action identity.Action, next string) string {
	query := url.Values{"for": {string(action)}, "next": {next}}
	return "/" + i18n.FromContext(r.Context()) + stepUpPath + "?" + query.Encode() + "&"
}

// stepUpPage opens a step-up challenge and asks for the second factor.
func (s Site) stepUpPage(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	action, next, ok := stepUpTarget(query.Get("for"), query.Get("next"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	session, _ := SessionFrom(r.Context())
	// A step-up already open for this action is shown again rather than
	// replaced, so a reload keeps its attempts and a code already sent.
	token := challengeToken(r)
	if !s.liveStepUp(r, token, session, action) {
		var err error
		token, err = s.identity.Service.BeginStepUp(r.Context(), visit(r), session, action)
		if errors.Is(err, identity.ErrNoSecondFactor) {
			http.NotFound(w, r)
			return
		}
		if err != nil {
			s.failed(w, r, "a step-up could not start", err)
			return
		}
		setChallenge(w, r, token)
	}
	form := web.Form{Error: stepUpAgain[query.Get("again")]}
	s.renderStepUp(w, r, http.StatusOK, token, action, next, query.Get("method"), form)
}

// liveStepUp reports whether token opened a step-up of session for action
// that can still be answered.
func (s Site) liveStepUp(r *http.Request, token string, session identity.Session, action identity.Action) bool {
	if token == "" {
		return false
	}
	pending, err := s.identity.Service.Pending(r.Context(), visit(r), token, session.ID)
	return err == nil && pending.Action == action
}

func (s Site) renderStepUp(w http.ResponseWriter, r *http.Request, status int, token string,
	action identity.Action, next, chosen string, form web.Form) {
	session, _ := SessionFrom(r.Context())
	pending, err := s.identity.Service.Pending(r.Context(), visit(r), token, session.ID)
	if err != nil {
		s.failed(w, r, "a step-up could not be read", err)
		return
	}
	view := challengeView(pending, chosen, stepUpBase(r, action, next))
	view.Next, view.Form, view.Sent = next, form, r.URL.Query().Get("sent") == "1"
	if err := s.keyOptions(r, &view, token, session.ID); err != nil {
		s.failed(w, r, "a key's options could not be prepared", err)
		return
	}
	page := s.page(r, stepUpPath)
	if r.Method == http.MethodPost {
		// A page shown in answer to a post has no query of its own: the
		// language switch is given the step-up's, with the method shown, so
		// that it returns to this step-up rather than to one that no longer
		// knows what it is for.
		page.Query = url.Values{"for": {string(action)}, "next": {next}, "method": {view.Method}}.Encode()
	}
	renderStatus(w, r, status, web.ChallengePage(page, view))
}

// answerStepUp checks the step-up's answer and returns where the step-up was
// asked from. A challenge that can no longer be answered starts a new one.
func (s Site) answerStepUp(w http.ResponseWriter, r *http.Request) {
	action, next, ok := stepUpTarget(r.PostFormValue("for"), r.PostFormValue("next"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	session, _ := SessionFrom(r.Context())
	token := challengeToken(r)
	answer := answerOf(r)
	err := s.identity.Service.StepUp(r.Context(), visit(r), session, token, answer)
	restart := func(why string) {
		query := url.Values{"for": {string(action)}, "next": {next}, "again": {why}}
		http.Redirect(w, r, "/"+i18n.FromContext(r.Context())+stepUpPath+"?"+query.Encode(), http.StatusSeeOther)
	}
	switch {
	case errors.Is(err, identity.ErrCodeWrong):
		s.renderStepUp(w, r, http.StatusUnauthorized, token, action, next, string(answer.Method),
			wrongAnswer(string(answer.Method)))
	case errors.Is(err, identity.ErrChallengeExhausted):
		restart("exhausted")
	case errors.Is(err, identity.ErrChallengeInvalid):
		restart("expired")
	case err != nil:
		s.failed(w, r, "a step-up failed", err)
	default:
		clearChallenge(w, r)
		// #nosec G710 -- stepUpTarget kept a path on this site and discarded any host (safePath).
		http.Redirect(w, r, "/"+i18n.FromContext(r.Context())+next, http.StatusSeeOther)
	}
}

// sendStepUpCode mails a code for the session's step-up, and shows the page
// again saying so.
func (s Site) sendStepUpCode(w http.ResponseWriter, r *http.Request) {
	action, next, ok := stepUpTarget(r.PostFormValue("for"), r.PostFormValue("next"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	session, _ := SessionFrom(r.Context())
	token := challengeToken(r)
	err := s.identity.Service.SendChallengeCode(r.Context(), visit(r), token, session.ID)
	if key, refused := codeRefusal(err); refused {
		s.renderStepUp(w, r, http.StatusTooManyRequests, token, action, next, string(identity.MethodEmail), web.Form{Error: key})
		return
	}
	query := url.Values{"for": {string(action)}, "next": {next}}
	switch {
	case errors.Is(err, identity.ErrChallengeInvalid):
		query.Set("again", "expired")
	case errors.Is(err, identity.ErrNotPermitted):
		http.NotFound(w, r)
		return
	case err != nil:
		s.failed(w, r, "a step-up's code could not be sent", err)
		return
	default:
		query.Set("method", string(identity.MethodEmail))
		query.Set("sent", "1")
	}
	http.Redirect(w, r, "/"+i18n.FromContext(r.Context())+stepUpPath+"?"+query.Encode(), http.StatusSeeOther)
}
