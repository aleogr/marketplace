package httpx_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/aleogr/marketplace/internal/identity"
	"github.com/aleogr/marketplace/internal/identity/breached"
	"github.com/aleogr/marketplace/internal/platform/httpx"
	"github.com/aleogr/marketplace/internal/platform/i18n"
	"github.com/aleogr/marketplace/internal/platform/ratelimit"
	"github.com/aleogr/marketplace/internal/platform/seo"
)

// withSession is a request from a browser whose session is session.
func withSession(r *http.Request, session identity.Session) *http.Request {
	return r.WithContext(httpx.WithSession(r.Context(), session))
}

// buyer is a buyer's session: with or without 2FA, stepped up at steppedUp
// or never.
func buyer(secondFactor bool, steppedUp *time.Time) identity.Session {
	return identity.Session{
		ID:           "00000000-0000-0000-0000-00000000000a",
		Account:      identity.Account{ID: "00000000-0000-0000-0000-00000000000b", Name: "Leitora", Kind: identity.KindBuyer},
		SecondFactor: secondFactor, SteppedUpAt: steppedUp,
	}
}

// guardedSite is a site whose guard the tests mount on a stub.
func guardedSite(t *testing.T) httpx.Site {
	t.Helper()
	catalogue, err := i18n.Load()
	if err != nil {
		t.Fatal(err)
	}
	preview, err := seo.NewPreview()
	if err != nil {
		t.Fatal(err)
	}
	never := ratelimit.Never{}
	return httpx.NewSite(nil, catalogue, preview, false).WithIdentity(httpx.IdentityRoutes{
		Service: identity.NewService(nil, identity.NewHasher(cheap, 1), breached.Fake{}, nil, silent()),
		Limits:  httpx.IdentityLimits{SignUp: never, Resend: never, ResendAddress: never, SignIn: never, SignInAddress: never, Password: never, StepUp: never},
		Pages:   httpx.NewPages(catalogue),
		Log:     silent(),
	})
}

// The step-up entry points of adding a card and changing the address: the
// guard sends a buyer to prove a second factor first, even without 2FA, and
// back to where they were; a recent step-up goes straight through (§18.2).
func TestTheStepUpGuardSendsABuyerToConfirmBeforeACard(t *testing.T) {
	served := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	for _, tc := range []struct {
		action identity.Action
		back   string
		want   string
	}{
		{identity.ActionAddCard, "/checkout/card", "/pt-BR/account/verify?for=add_card&next=%2Fcheckout%2Fcard"},
		{identity.ActionChangeEmail, "/account/email", "/pt-BR/account/verify?for=change_email&next=%2Faccount%2Femail"},
	} {
		t.Run(string(tc.action), func(t *testing.T) {
			guard := guardedSite(t).SteppedUp(tc.action, tc.back, served)
			request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, tc.back, nil)
			recorder := httptest.NewRecorder()
			guard.ServeHTTP(recorder, withSession(inMarketplace(request), buyer(false, nil)))
			if recorder.Code != http.StatusSeeOther || recorder.Header().Get("Location") != tc.want {
				t.Fatalf("status %d, Location %q; want %d to %s", recorder.Code, recorder.Header().Get("Location"),
					http.StatusSeeOther, tc.want)
			}

			now := time.Now().UTC()
			recorder = httptest.NewRecorder()
			guard.ServeHTTP(recorder, withSession(inMarketplace(request), buyer(false, &now)))
			if recorder.Code != http.StatusNoContent {
				t.Fatalf("after a recent step-up: status %d, want the page", recorder.Code)
			}
		})
	}
}

// The password page asks whoever has a second factor to prove it first
// (D2), and nobody else.
func TestThePasswordPageAsksForTheSecondFactorFirst(t *testing.T) {
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/account/password", nil)
	recorder := httptest.NewRecorder()
	identitySite(t).ServeHTTP(recorder, withSession(inMarketplace(request), buyer(true, nil)))
	if want := "/pt-BR/account/verify?for=password&next=%2Faccount%2Fpassword"; recorder.Code != http.StatusSeeOther ||
		recorder.Header().Get("Location") != want {
		t.Fatalf("with 2FA: status %d, Location %q; want %d to %s", recorder.Code, recorder.Header().Get("Location"),
			http.StatusSeeOther, want)
	}
	recorder = httptest.NewRecorder()
	identitySite(t).ServeHTTP(recorder, withSession(inMarketplace(request), buyer(false, nil)))
	if recorder.Code != http.StatusOK {
		t.Fatalf("without 2FA: status %d, want the page", recorder.Code)
	}
}

// A step-up for something the policy does not name is not found, before the
// service is asked anything (identitySite has no database).
func TestAStepUpForAnUnknownActionIsNotFound(t *testing.T) {
	get := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/account/verify?for=delete_everything&next=%2F", nil)
	recorder := httptest.NewRecorder()
	identitySite(t).ServeHTTP(recorder, withSession(inMarketplace(get), buyer(true, nil)))
	if recorder.Code != http.StatusNotFound {
		t.Errorf("GET: status %d, want %d", recorder.Code, http.StatusNotFound)
	}
	post := formRequest(t, "/account/verify", url.Values{"for": {"delete_everything"}, "next": {"/"}, "code": {"123456"}})
	recorder = httptest.NewRecorder()
	identitySite(t).ServeHTTP(recorder, withSession(post, buyer(true, nil)))
	if recorder.Code != http.StatusNotFound {
		t.Errorf("POST: status %d, want %d", recorder.Code, http.StatusNotFound)
	}
}

// The answers to step-ups are limited per account, and only they are.
func TestStepUpAnswersAreLimitedPerAccount(t *testing.T) {
	handler := identityHandler(t,
		identity.NewService(nil, identity.NewHasher(cheap, 1), breached.Fake{}, nil, silent()),
		limitsRefusing("StepUp"))
	post := formRequest(t, "/account/verify", url.Values{"for": {"factors"}, "next": {"/account/security"}, "code": {"123456"}})
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, withSession(post, buyer(true, nil)))
	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("status %d, want %d", recorder.Code, http.StatusTooManyRequests)
	}
}
