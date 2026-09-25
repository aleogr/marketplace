package httpx_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/aleogr/marketplace/internal/identity"
	"github.com/aleogr/marketplace/internal/identity/breached"
)

// The second step counts against the sign-in limits, per client and per
// address, as the password does; with no challenge to answer it goes back to
// the password, before the service is asked anything (identityHandler's
// service has no database, so reaching it would panic).
func TestTheSecondStepCountsAgainstTheSignInLimits(t *testing.T) {
	for _, slot := range []string{"SignIn", "SignInAddress", "SignUp", "Password"} {
		t.Run(slot, func(t *testing.T) {
			handler := identityHandler(t,
				identity.NewService(nil, identity.NewHasher(cheap, 1), breached.Fake{}, nil, silent()),
				limitsRefusing(slot))
			for _, path := range []string{"/signin/verify", "/signin/verify/email"} {
				recorder := httptest.NewRecorder()
				handler.ServeHTTP(recorder, formRequest(t, path, url.Values{"method": {"totp"}, "code": {"123456"}}))
				switch slot {
				case "SignIn", "SignInAddress":
					if recorder.Code != http.StatusTooManyRequests {
						t.Errorf("%s: status = %d, want %d", path, recorder.Code, http.StatusTooManyRequests)
					}
				default:
					if recorder.Code != http.StatusSeeOther || recorder.Header().Get("Location") != "/pt-BR/signin?again=expired" {
						t.Errorf("%s: status %d, Location %q; want %d to /pt-BR/signin?again=expired",
							path, recorder.Code, recorder.Header().Get("Location"), http.StatusSeeOther)
					}
				}
			}
		})
	}
}

// Reading the second step with no challenge goes back to the password.
func TestTheSecondStepWithNoChallengeGoesBackToSignIn(t *testing.T) {
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/signin/verify", nil)
	recorder := httptest.NewRecorder()
	identitySite(t).ServeHTTP(recorder, inMarketplace(request))
	if recorder.Code != http.StatusSeeOther || recorder.Header().Get("Location") != "/pt-BR/signin?again=expired" {
		t.Fatalf("status %d, Location %q", recorder.Code, recorder.Header().Get("Location"))
	}
}

// The sign-in page says why a second step sent the visitor back, and says
// nothing for a reason it does not know.
func TestTheSignInPageSaysWhyTheSecondStepEnded(t *testing.T) {
	for query, want := range map[string]string{
		"again=expired":   "A confirmação expirou. Entre de novo.",
		"again=exhausted": "Muitos códigos errados. Entre de novo.",
	} {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/signin?"+query, nil)
		recorder := httptest.NewRecorder()
		identitySite(t).ServeHTTP(recorder, inMarketplace(request))
		if !strings.Contains(recorder.Body.String(), want) {
			t.Errorf("%s: the page does not say %q", query, want)
		}
	}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/signin?again=whatever", nil)
	recorder := httptest.NewRecorder()
	identitySite(t).ServeHTTP(recorder, inMarketplace(request))
	if strings.Contains(recorder.Body.String(), `id="form-error"`) {
		t.Error("an unknown reason was shown")
	}
}
