package i18n

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/aleogr/marketplace/internal/platform/geoip"
)

// Cookie is where a visitor's own choice is remembered.
//
// A year, because the choice is a preference and not a session: a visitor who
// chose Portuguese in March should not be asked again in April
// (docs/design.md, decision 10).
const (
	Cookie    = "language"
	cookieAge = 365 * 24 * time.Hour
)

type languageKey struct{}

// FromContext returns the language resolved for the request.
func FromContext(ctx context.Context) string {
	tag, _ := ctx.Value(languageKey{}).(string)
	if tag == "" {
		return Default
	}
	return tag
}

// WithLanguage returns a context carrying tag.
func WithLanguage(ctx context.Context, tag string) context.Context {
	return context.WithValue(ctx, languageKey{}, tag)
}

// Resolver decides which language a request is served in.
//
// The order is decision 10 of docs/design.md, and every step of it exists
// because the one after it is a guess:
//
//  1. the language in the address, which always wins — a visitor in Brazil who
//     opens /en-US/ gets English, and a search engine indexing that address
//     gets English too;
//  2. the choice the visitor made before, remembered in a cookie;
//  3. the country the address is in, which is a guess about a person from
//     where their packets came from;
//  4. en-US, the official language.
//
// Only the first is in the address, so the other three end in a redirect: the
// language is part of the URL, for indexing, and a page served at an address
// without one would be a page search engines see in whichever language the
// crawler happened to be guessed into.
type Resolver struct {
	catalogue *Catalogue
	locator   geoip.Locator
	// countries maps a country to the language spoken there. Portuguese for
	// Brazil is the one the requirements name; a market added later adds a
	// line here and a catalogue.
	countries map[string]string
}

// NewResolver returns a resolver using catalogue and locator.
func NewResolver(catalogue *Catalogue, locator geoip.Locator) *Resolver {
	return &Resolver{
		catalogue: catalogue,
		locator:   locator,
		countries: map[string]string{"BR": "pt-BR"},
	}
}

// Enabled reports which languages a request's marketplace serves, and which of
// them is its default. Returning no languages means the platform's own: this is
// how the platform host, which belongs to no marketplace, is served.
type Enabled func(r *http.Request) (languages []string, fallback string)

// Resolve mounts the resolver, exempting the paths given — the health check and
// anything else served to a machine rather than to a person.
func (rs *Resolver) Resolve(enabled Enabled, exempt ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			for _, path := range exempt {
				if r.URL.Path == path {
					next.ServeHTTP(w, r)
					return
				}
			}

			languages, fallback := rs.speaks(enabled, r)

			if head, rest, ok := splitLanguage(r.URL.Path); ok {
				if canonical, known := match(head, languages); known {
					if canonical != head {
						// A lowercase address is a different address to a
						// search engine, so it is answered once, permanently,
						// with the canonical one (docs/design.md, decision 10).
						redirect(w, r, http.StatusMovedPermanently, canonical, rest)
						return
					}
					r.URL.Path = rest
					next.ServeHTTP(w, r.WithContext(WithLanguage(r.Context(), canonical)))
					return
				}
			}

			// No language in the address: choose one and say so in the
			// address, rather than serving two languages at one URL.
			redirect(w, r, http.StatusFound, rs.choose(r, languages, fallback), r.URL.Path)
		})
	}
}

// speaks returns the languages this request may be served in, and which to fall
// back to, bounded by what the platform has catalogues for.
func (rs *Resolver) speaks(enabled Enabled, r *http.Request) ([]string, string) {
	languages, fallback := enabled(r)
	if len(languages) == 0 {
		languages = rs.catalogue.Languages()
	}

	spoken := make([]string, 0, len(languages))
	for _, tag := range languages {
		if rs.catalogue.Speaks(tag) {
			spoken = append(spoken, tag)
		}
	}
	if len(spoken) == 0 {
		spoken = []string{Default}
	}

	if !contains(spoken, fallback) {
		fallback = Default
		if !contains(spoken, fallback) {
			fallback = spoken[0]
		}
	}
	return spoken, fallback
}

// choose applies the order of decision 10 to a request with no language in it.
func (rs *Resolver) choose(r *http.Request, languages []string, fallback string) string {
	if cookie, err := r.Cookie(Cookie); err == nil {
		if chosen, ok := match(cookie.Value, languages); ok {
			return chosen
		}
	}

	if ip := clientIP(r); ip != "" {
		if country, known := rs.locator.Country(ip); known {
			if spoken, ok := rs.countries[country]; ok {
				if chosen, ok := match(spoken, languages); ok {
					return chosen
				}
			}
		}
	}

	return fallback
}

// Remember writes the visitor's choice, which every later visit obeys.
func Remember(w http.ResponseWriter, r *http.Request, tag string) {
	// Secure follows the scheme of the request: a deployment is reached over
	// HTTPS and gets a Secure cookie, while a local process and the
	// end-to-end suite are reached over plain HTTP, where a browser would
	// discard one and the choice could not be remembered at all.
	// #nosec G124 -- the flags follow the scheme of the request, see above.
	http.SetCookie(w, &http.Cookie{
		Name:     Cookie,
		Value:    tag,
		Path:     "/",
		MaxAge:   int(cookieAge.Seconds()),
		HttpOnly: true,
		Secure:   r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https"),
		SameSite: http.SameSiteLaxMode,
	})
}

// splitLanguage takes the first segment of a path, which is where a language
// tag would be.
func splitLanguage(path string) (head, rest string, ok bool) {
	trimmed := strings.TrimPrefix(path, "/")
	head, rest, found := strings.Cut(trimmed, "/")
	if head == "" {
		return "", "/", false
	}
	if !found {
		return head, "/", true
	}
	return head, "/" + rest, true
}

// match finds the canonical spelling of a tag among the languages given,
// ignoring case: `pt-br` and `PT-BR` are the same language as `pt-BR`, spelled
// differently, and only one of the three is the address to index.
func match(tag string, languages []string) (string, bool) {
	for _, language := range languages {
		if strings.EqualFold(tag, language) {
			return language, true
		}
	}
	return "", false
}

func contains(languages []string, tag string) bool {
	_, ok := match(tag, languages)
	return ok
}

// redirect sends the visitor to the same place, in a language.
//
// The target is built as a URL with a path and nothing else — no scheme and no
// host — so what comes out is an address on this site whatever the path was.
// That is what makes this not an open redirect: `tag` is one of the languages
// matched above, and the path is the one this request already asked for, kept
// exactly as it was down to its trailing slash, because a path that ends in
// one is a different address to a search engine.
func redirect(w http.ResponseWriter, r *http.Request, status int, tag, path string) {
	target := url.URL{Path: "/" + tag + path, RawQuery: r.URL.RawQuery}
	// #nosec G710 -- the target is a path on this site, under a language this
	// server chose; see above.
	http.Redirect(w, r, target.String(), status)
}

// clientIP reads what the origin middleware left, without importing it: the
// header is the same one, and a package that speaks languages has no business
// depending on one that counts requests.
func clientIP(r *http.Request) string {
	if origin, ok := r.Context().Value(originIP{}).(string); ok {
		return origin
	}
	return ""
}

// originIP is the key the HTTP layer stores the client address under for this
// package's benefit.
type originIP struct{}

// WithClientIP returns a context carrying the address language resolution
// should ask the locator about.
func WithClientIP(ctx context.Context, ip string) context.Context {
	return context.WithValue(ctx, originIP{}, ip)
}
