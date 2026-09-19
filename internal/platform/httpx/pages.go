package httpx

import (
	"net/http"
	"time"

	"github.com/aleogr/marketplace/internal/platform/i18n"
	"github.com/aleogr/marketplace/internal/platform/ratelimit"
	"github.com/aleogr/marketplace/internal/tenancy"
	"github.com/aleogr/marketplace/web"
)

// Pages answers the requests that never reach a marketplace's own handlers.
//
// Every string here comes from a catalogue, like every other string a person
// reads (docs/requirements.md, section 6).
type Pages struct {
	catalogue *i18n.Catalogue
}

// NewPages returns the pages, speaking catalogue.
func NewPages(catalogue *i18n.Catalogue) Pages { return Pages{catalogue: catalogue} }

// UnknownHost answers a request whose host belongs to no marketplace.
//
// In every language, and that is not indecision: the platform picks a language
// from the marketplace, the URL or the visitor's country, and this request has
// no marketplace to pick from and no language in its address. Guessing one
// would be a worse answer than showing them all.
func (p Pages) UnknownHost(w http.ResponseWriter, _ *http.Request) {
	// Not indexable, whatever the deployment's setting: an address nobody
	// configured must never become a page in a search engine.
	w.Header().Set("X-Robots-Tag", "noindex, nofollow")
	p.everyLanguage(w, http.StatusNotFound, "error.unknown_host")
}

// InPreparation answers a request for a marketplace that does not serve yet.
//
// It answers rather than refuses, on purpose: a marketplace leaves
// `in_preparation` only once its host is shown to answer, so a host that
// refused every request could never be activated (docs/design.md, decision 16).
//
// Here the language is known, because the marketplace is: this runs before
// language resolution — it is the answer to whether there is a marketplace at
// all — so it is served in the marketplace's own default language.
func (p Pages) InPreparation(w http.ResponseWriter, r *http.Request, marketplace *tenancy.Marketplace) {
	w.Header().Set("X-Robots-Tag", "noindex, nofollow")

	tag := marketplace.DefaultLanguage
	page := web.NewPage(tag, p.catalogue.Printer(tag), nil)
	page.Marketplace = marketplace.Name
	page.Nonce = Nonce(r.Context())
	page.Default = "/" + i18n.Default + "/"

	render(w, r, web.Preparing(page))
}

// forbidden answers a request that could not prove where it came from.
func (p Pages) forbidden(w http.ResponseWriter) {
	p.everyLanguage(w, http.StatusForbidden, "error.forbidden")
}

// Refused answers a request that has come too often.
//
// It is exported because the rate limiter mounts it, and the limiter knows
// nothing about pages (internal/platform/ratelimit).
func (p Pages) Refused(w http.ResponseWriter, _ *http.Request, retryAfter time.Duration) {
	ratelimit.RetryAfter(w, retryAfter)
	p.everyLanguage(w, http.StatusTooManyRequests, "error.too_many_requests")
}

// everyLanguage writes a short message in every language the platform speaks.
//
// These are the answers given before a language is resolved — a refusal
// happens on the way in, not on the way to a page — so there is nothing to
// resolve one from.
func (p Pages) everyLanguage(w http.ResponseWriter, status int, key string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(status)

	for _, language := range p.catalogue.Languages() {
		_, _ = w.Write([]byte(p.catalogue.Printer(language).Sprintf(key) + "\n"))
	}
}
