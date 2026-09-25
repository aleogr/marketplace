//go:build integration

package httpx_test

import (
	"html"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/aleogr/marketplace/internal/identity"
	"github.com/aleogr/marketplace/internal/identity/breached"
	"github.com/aleogr/marketplace/internal/platform/audit"
	"github.com/aleogr/marketplace/internal/platform/config"
	"github.com/aleogr/marketplace/internal/platform/db"
	"github.com/aleogr/marketplace/internal/platform/dbtest"
	"github.com/aleogr/marketplace/internal/platform/geoip"
	"github.com/aleogr/marketplace/internal/platform/httpx"
	"github.com/aleogr/marketplace/internal/platform/i18n"
	"github.com/aleogr/marketplace/internal/platform/keys/local"
	"github.com/aleogr/marketplace/internal/platform/ratelimit"
	"github.com/aleogr/marketplace/internal/tenancy"
	"github.com/jackc/pgx/v5"
)

// The language switch's hidden fields, as the header's form carries them.
var (
	pathOnPage  = regexp.MustCompile(`<input type="hidden" name="path" value="([^"]*)">`)
	queryOnPage = regexp.MustCompile(`<input type="hidden" name="query" value="([^"]*)">`)
)

// bilingualSite is the identity routes behind the session middleware and the
// language resolver, as the process mounts them, for a marketplace of the
// test's own that speaks pt-BR and en-US, with one confirmed account.
func bilingualSite(t *testing.T) (http.Handler, *identity.Service, *tenancy.Marketplace) {
	t.Helper()
	pool, err := db.Open(t.Context(), config.Database{URL: dbtest.URL(t)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	dbtest.AsApplication(t, pool)

	slug := unique(t, "bilingue")
	languages := []string{"pt-BR", "en-US"}
	spec := tenancy.Spec{Slug: slug, Name: "Loja", Market: "BR", RevenueModel: "commission", State: "active",
		DefaultLanguage: "pt-BR", Languages: languages, Hosts: []string{slug + ".test"}}
	var applied []tenancy.Applied
	if err := pool.InTx(t.Context(), func(tx pgx.Tx) error {
		applied, err = tenancy.Seed(t.Context(), tx, []tenancy.Spec{spec})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	marketplace := &tenancy.Marketplace{ID: applied[0].ID, Name: spec.Name, State: tenancy.Active,
		DefaultLanguage: "pt-BR", Languages: languages}

	keeper, err := local.Generate()
	if err != nil {
		t.Fatal(err)
	}
	service := identity.NewService(serving{t, pool}, identity.NewHasher(cheap, 2), breached.Fake{}, unaudited{}, silent()).
		WithSealer(audit.NewKeys(keeper))
	catalogue, err := i18n.Load()
	if err != nil {
		t.Fatal(err)
	}
	never := ratelimit.Never{}
	handler := i18n.NewResolver(catalogue, geoip.Nowhere{}).Resolve(httpx.Speaks(catalogue), httpx.LanguagePath)(
		httpx.Sessions(service, silent())(identityHandler(t, service, httpx.IdentityLimits{
			SignUp: never, Resend: never, ResendAddress: never, SignIn: never, SignInAddress: never, Password: never,
			StepUp: never,
		})))
	confirmedAccount(t, pool, service, marketplace, "Leitora", "leitora@example.test", "correct horse battery staple")
	return handler, service, marketplace
}

// hidden is the value of the language switch's hidden field on page, as a
// browser would post it.
func hidden(t *testing.T, field *regexp.Regexp, page string) string {
	t.Helper()
	found := field.FindStringSubmatch(page)
	if found == nil {
		t.Fatalf("the language switch carries no %s: %s", field, page)
	}
	return html.UnescapeString(found[1])
}

// Switching the language halfway through a step-up stays on the step-up, for
// the same action: the page's query — what the step-up is for and where it
// returns to — goes through the switch with its path.
func TestTheLanguageSwitchKeepsTheStepUp(t *testing.T) {
	handler, service, marketplace := bilingualSite(t)
	b := signedInBrowser(t, handler, service, marketplace)
	enrolledApp(t, service, marketplace)

	asked := b.get("/pt-BR/account/verify?for=password&next=%2Faccount%2Fpassword")
	body := asked.Body.String()
	if asked.Code != http.StatusOK || !strings.Contains(body, "Antes de alterar sua senha, confirme que é você.") {
		t.Fatalf("the step-up page in pt-BR: status %d, body %s", asked.Code, body)
	}

	switched := b.post(httpx.LanguagePath, url.Values{"language": {"en-US"},
		"path": {hidden(t, pathOnPage, body)}, "query": {hidden(t, queryOnPage, body)}})
	location := switched.Header().Get("Location")
	if switched.Code != http.StatusSeeOther || !strings.HasPrefix(location, "/en-US/account/verify?") {
		t.Fatalf("the switch: status %d, Location %q", switched.Code, location)
	}

	again := b.get(location)
	body = again.Body.String()
	if again.Code != http.StatusOK || !strings.Contains(body, "Before changing your password, confirm it is you.") ||
		!strings.Contains(body, `<html lang="en-US"`) {
		t.Fatalf("the step-up page after the switch: status %d, body %s", again.Code, body)
	}
}
