package httpx

import (
	"errors"
	"net/http"
	"net/url"
	"slices"

	"github.com/aleogr/marketplace/internal/identity"
	"github.com/aleogr/marketplace/internal/platform/i18n"
	"github.com/aleogr/marketplace/web"
)

// The challenge cookie carries the token of a second step in progress
// (F14 spec, D3). It follows the session cookie's rules: `__Host-` over
// HTTPS, never readable by script, and it lives as long as the challenge.
const (
	challengeCookie         = "__Host-challenge"
	challengeCookieInsecure = "challenge"
)

func challengeCookieName(r *http.Request) string {
	if isHTTPS(r) {
		return challengeCookie
	}
	return challengeCookieInsecure
}

// challengeToken is the token the request's challenge cookie carries, or "".
func challengeToken(r *http.Request) string {
	if c, err := r.Cookie(challengeCookieName(r)); err == nil {
		return c.Value
	}
	return ""
}

func setChallenge(w http.ResponseWriter, r *http.Request, token string) {
	// #nosec G124 -- Secure follows the scheme, as the session cookie's does (session.go).
	http.SetCookie(w, &http.Cookie{
		Name: challengeCookieName(r), Value: token, Path: "/", HttpOnly: true,
		Secure: isHTTPS(r), SameSite: http.SameSiteLaxMode,
		MaxAge: int(identity.ChallengeLifetime.Seconds()),
	})
}

func clearChallenge(w http.ResponseWriter, r *http.Request) {
	// #nosec G124 -- see setChallenge.
	http.SetCookie(w, &http.Cookie{
		Name: challengeCookieName(r), Value: "", Path: "/", HttpOnly: true,
		Secure: isHTTPS(r), SameSite: http.SameSiteLaxMode, MaxAge: -1,
	})
}

// byChallenge limits the second step by the address its challenge is for, in
// this marketplace: the same subject byAddress gives the password step, so
// the F13 per-address limit covers both steps together.
func (s Site) byChallenge(r *http.Request) string {
	token := challengeToken(r)
	if token == "" {
		return marketplaceOf(r) + ":"
	}
	return marketplaceOf(r) + ":" + s.identity.Service.ChallengeAddress(r.Context(), visit(r), token)
}

// again is what the sign-in page may say when a second step sent the visitor
// back: the query names it, and only these keys are shown.
var again = map[string]string{
	"expired":   "identity.challenge.expired",
	"exhausted": "identity.challenge.exhausted",
}

// backToSignIn ends a second step that can no longer be answered.
func backToSignIn(w http.ResponseWriter, r *http.Request, why string) {
	clearChallenge(w, r)
	http.Redirect(w, r, "/"+i18n.FromContext(r.Context())+"/signin?again="+why, http.StatusSeeOther)
}

// shownMethod is the method a challenge page shows: the one the visitor
// chose, when the challenge accepts it, or the strongest.
func shownMethod(pending identity.Pending, chosen string) string {
	method := identity.Method(chosen)
	if slices.Contains(pending.Methods, method) || (method == identity.MethodRecovery && pending.Recovery) {
		return chosen
	}
	if len(pending.Methods) > 0 {
		return string(pending.Methods[0])
	}
	return string(identity.MethodRecovery)
}

// challengeView is what a challenge page shows for pending.
func challengeView(pending identity.Pending, chosen, base string) web.Challenge {
	view := web.Challenge{Action: string(pending.Action), Recovery: pending.Recovery, Base: base,
		Method: shownMethod(pending, chosen)}
	for _, method := range pending.Methods {
		view.Methods = append(view.Methods, string(method))
	}
	return view
}

// secondStepPath is where the second step of a sign-in lives.
const secondStepPath = "/signin/verify"

// secondStep shows the second step of a sign-in, strongest method first.
func (s Site) secondStep(w http.ResponseWriter, r *http.Request) {
	s.renderSecondStep(w, r, http.StatusOK, r.URL.Query().Get("method"), web.Form{})
}

func (s Site) renderSecondStep(w http.ResponseWriter, r *http.Request, status int, chosen string, form web.Form) {
	token := challengeToken(r)
	if token == "" {
		backToSignIn(w, r, "expired")
		return
	}
	pending, err := s.identity.Service.Pending(r.Context(), visit(r), token, "")
	if errors.Is(err, identity.ErrChallengeInvalid) {
		backToSignIn(w, r, "expired")
		return
	}
	if err != nil {
		s.failed(w, r, "a second step could not be read", err)
		return
	}
	view := challengeView(pending, chosen, "/"+i18n.FromContext(r.Context())+secondStepPath+"?")
	view.Form, view.Sent = form, r.URL.Query().Get("sent") == "1"
	if err := s.keyOptions(r, &view, token, ""); err != nil {
		s.failed(w, r, "a key's options could not be prepared", err)
		return
	}
	page := s.page(r, secondStepPath)
	if r.Method == http.MethodPost {
		// As the step-up's: the language switch keeps the method shown.
		page.Query = url.Values{"method": {view.Method}}.Encode()
	}
	renderStatus(w, r, status, web.ChallengePage(page, view))
}

// keyOptions gives a challenge page that shows a key what the browser needs
// to answer with it: each showing starts a new ceremony.
func (s Site) keyOptions(r *http.Request, view *web.Challenge, token, sessionID string) error {
	if view.Method != string(identity.MethodKey) {
		return nil
	}
	options, err := s.identity.Service.KeyOptions(r.Context(), visit(r), token, sessionID)
	view.KeyOptions = string(options)
	return err
}

// answerOf is the answer a challenge's form posted: a code, or a key's.
func answerOf(r *http.Request) identity.Answer {
	return identity.Answer{Method: identity.Method(r.PostFormValue("method")), Code: r.PostFormValue("code"),
		Key: []byte(r.PostFormValue("key"))}
}

// wrongAnswer is what a refused answer is shown as: a code that is not
// right, or a key whose answer did not verify.
func wrongAnswer(method string) web.Form {
	if method == string(identity.MethodKey) {
		return web.Form{Error: "identity.key.refused"}
	}
	return web.Form{Error: "identity.challenge.wrong", Field: "code"}
}

// codeRefusal names the message a refused request for an e-mail code is
// shown as.
func codeRefusal(err error) (string, bool) {
	switch {
	case errors.Is(err, identity.ErrCodeTooSoon):
		return "identity.email.too_soon", true
	case errors.Is(err, identity.ErrCodeTooMany):
		return "identity.email.too_many", true
	}
	return "", false
}

// sendSecondStepCode mails a code for the sign-in's second step, and shows
// the page again saying so.
func (s Site) sendSecondStepCode(w http.ResponseWriter, r *http.Request) {
	token := challengeToken(r)
	if token == "" {
		backToSignIn(w, r, "expired")
		return
	}
	err := s.identity.Service.SendChallengeCode(r.Context(), visit(r), token, "")
	if key, refused := codeRefusal(err); refused {
		s.renderSecondStep(w, r, http.StatusTooManyRequests, string(identity.MethodEmail), web.Form{Error: key})
		return
	}
	switch {
	case errors.Is(err, identity.ErrChallengeInvalid):
		backToSignIn(w, r, "expired")
	case errors.Is(err, identity.ErrNotPermitted):
		http.NotFound(w, r)
	case err != nil:
		s.failed(w, r, "a second step's code could not be sent", err)
	default:
		http.Redirect(w, r, "/"+i18n.FromContext(r.Context())+secondStepPath+"?method=email&sent=1", http.StatusSeeOther)
	}
}

// answerSecondStep completes a sign-in. A right answer opens the session as a
// sign-in without a second factor does: the session the browser held is
// revoked, the cookie replaced and the CSRF secret renewed. A recovery code
// goes on to the security page, which says how many are left.
func (s Site) answerSecondStep(w http.ResponseWriter, r *http.Request) {
	token := challengeToken(r)
	if token == "" {
		backToSignIn(w, r, "expired")
		return
	}
	answer := answerOf(r)
	signed, err := s.identity.Service.CompleteSignIn(r.Context(), visit(r), token, answer)
	switch {
	case errors.Is(err, identity.ErrCodeWrong):
		s.renderSecondStep(w, r, http.StatusUnauthorized, string(answer.Method), wrongAnswer(string(answer.Method)))
		return
	case errors.Is(err, identity.ErrChallengeExhausted):
		backToSignIn(w, r, "exhausted")
		return
	case errors.Is(err, identity.ErrChallengeInvalid):
		backToSignIn(w, r, "expired")
		return
	case err != nil:
		s.failed(w, r, "a second step failed", err)
		return
	}
	s.openSession(w, r, signed.Session)
	clearChallenge(w, r)
	target := "/" + i18n.FromContext(r.Context()) + "/"
	if signed.Recovery {
		target = "/" + i18n.FromContext(r.Context()) + securityPath + "?" +
			url.Values{"done": {"recovery_used"}}.Encode()
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
}

// openSession hands the browser a session it has just earned. The session
// it held, even another account's, is ended on the server rather than left
// valid with nobody holding its cookie, and the CSRF secret is renewed.
func (s Site) openSession(w http.ResponseWriter, r *http.Request, token string) {
	if held, err := r.Cookie(sessionCookieName(r)); err == nil && held.Value != "" {
		if err := s.identity.Service.Supersede(r.Context(), visit(r), held.Value); err != nil {
			s.identity.Log.ErrorContext(r.Context(), "a replaced session could not be revoked", "error", err)
		}
	}
	setSession(w, r, token)
	RenewCSRF(w, r)
}
