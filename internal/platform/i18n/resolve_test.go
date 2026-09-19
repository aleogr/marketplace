package i18n_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aleogr/marketplace/internal/platform/geoip"
	"github.com/aleogr/marketplace/internal/platform/i18n"
)

// resolution is what one request produced: the language a handler was given,
// or the address it was sent to instead.
type resolution struct {
	status   int
	location string
	language string
	path     string
}

// ask sends one request through the resolver.
func ask(t *testing.T, target string, locator geoip.Locator, cookie string, languages []string, fallback string) resolution {
	t.Helper()

	resolver := i18n.NewResolver(loaded(t), locator)

	var got resolution
	handler := resolver.Resolve(func(*http.Request) ([]string, string) {
		return languages, fallback
	}, "/health")(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got.language = i18n.FromContext(r.Context())
		got.path = r.URL.Path
	}))

	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "https://marketplace1.example"+target, nil)
	request = request.WithContext(i18n.WithClientIP(request.Context(), "203.0.113.7"))
	if cookie != "" {
		request.AddCookie(&http.Cookie{Name: i18n.Cookie, Value: cookie})
	}

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	got.status = recorder.Code
	got.location = recorder.Header().Get("Location")
	return got
}

// The marketplace of these tests speaks both languages and defaults to
// Portuguese, which is the lab's marketplace 1.
var both = []string{"en-US", "pt-BR"}

// TestTheAddressAlwaysWins is the rule the requirements state outright: a
// visitor from Brazil who opens /en-US/ sees English (section 6).
func TestTheAddressAlwaysWins(t *testing.T) {
	got := ask(t, "/en-US/", geoip.Fixed("BR"), "pt-BR", both, "pt-BR")

	if got.status != http.StatusOK {
		t.Fatalf("status = %d (%s), want the page itself", got.status, got.location)
	}
	if got.language != "en-US" {
		t.Errorf("language = %q; a contrary cookie and a Brazilian address both lost to the URL", got.language)
	}
}

func TestTheRememberedChoiceBeatsTheCountry(t *testing.T) {
	got := ask(t, "/", geoip.Fixed("US"), "pt-BR", both, "en-US")

	if got.status != http.StatusFound {
		t.Fatalf("status = %d, want a redirect", got.status)
	}
	if got.location != "/pt-BR/" {
		t.Errorf("Location = %q, want /pt-BR/", got.location)
	}
}

func TestTheCountryDecidesWhenNothingIsRemembered(t *testing.T) {
	got := ask(t, "/", geoip.Fixed("BR"), "", both, "en-US")

	if got.location != "/pt-BR/" {
		t.Errorf("a visitor from Brazil was sent to %q, want /pt-BR/", got.location)
	}
}

func TestTheOfficialLanguageIsTheLastWord(t *testing.T) {
	got := ask(t, "/", geoip.Nowhere{}, "", both, "en-US")

	if got.location != "/en-US/" {
		t.Errorf("a visitor nothing is known about was sent to %q, want /en-US/", got.location)
	}
}

// TestALowercaseAddressIsRedirectedOnce covers what a search engine sees: two
// spellings of one address are two pages, so one of them answers permanently
// with the other (docs/design.md, decision 10).
func TestALowercaseAddressIsRedirectedOnce(t *testing.T) {
	got := ask(t, "/pt-br/offers", geoip.Nowhere{}, "", both, "en-US")

	if got.status != http.StatusMovedPermanently {
		t.Errorf("status = %d, want 301", got.status)
	}
	if got.location != "/pt-BR/offers" {
		t.Errorf("Location = %q, want /pt-BR/offers", got.location)
	}
}

func TestAnUnknownTagIsNotALanguage(t *testing.T) {
	// `catalog` is a path, not a language, and must be served as one.
	got := ask(t, "/catalog", geoip.Nowhere{}, "", both, "en-US")

	if got.status != http.StatusFound {
		t.Fatalf("status = %d, want a redirect into a language", got.status)
	}
	if got.location != "/en-US/catalog" {
		t.Errorf("Location = %q, want /en-US/catalog", got.location)
	}
}

// TestALanguageTheMarketplaceDoesNotServeIsNotOffered: a marketplace chooses
// its languages, and an address in one it does not serve is not its page.
func TestALanguageTheMarketplaceDoesNotServeIsNotOffered(t *testing.T) {
	onlyPortuguese := []string{"pt-BR"}

	got := ask(t, "/en-US/", geoip.Nowhere{}, "", onlyPortuguese, "pt-BR")
	if got.location != "/pt-BR/en-US/" {
		t.Errorf("Location = %q, want the path served in the language the marketplace speaks", got.location)
	}

	// And its own cookie cannot conjure it either.
	got = ask(t, "/", geoip.Nowhere{}, "en-US", onlyPortuguese, "pt-BR")
	if got.location != "/pt-BR/" {
		t.Errorf("Location = %q, want /pt-BR/", got.location)
	}
}

func TestThePathReachesTheHandlerWithoutTheLanguage(t *testing.T) {
	got := ask(t, "/pt-BR/offers/17", geoip.Nowhere{}, "", both, "en-US")

	if got.status != http.StatusOK {
		t.Fatalf("status = %d (%s)", got.status, got.location)
	}
	if got.path != "/offers/17" {
		t.Errorf("the handler was given %q, want /offers/17: routes should not repeat the language", got.path)
	}
}

func TestTheHealthCheckIsNotRedirected(t *testing.T) {
	got := ask(t, "/health", geoip.Nowhere{}, "", both, "en-US")

	if got.status != http.StatusOK {
		t.Errorf("the health check answered %d; a probe follows no redirects and speaks no language", got.status)
	}
}

func TestAQuerySurvivesTheRedirect(t *testing.T) {
	got := ask(t, "/search?q=bicycle", geoip.Nowhere{}, "", both, "en-US")

	if got.location != "/en-US/search?q=bicycle" {
		t.Errorf("Location = %q, want the query kept", got.location)
	}
}
