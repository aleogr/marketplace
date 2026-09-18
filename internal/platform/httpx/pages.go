package httpx

import (
	"net/http"

	"github.com/aleogr/marketplace/internal/tenancy"
)

// Pages answers the requests that never reach a marketplace's own handlers.
//
// The texts are here rather than in a translation file because the translation
// mechanism arrives with F8 and these two pages exist now (docs/roadmap.md).
// When it lands, these are the first strings to move; the shape below — one
// entry per language, never a string in the handler — is what makes that a
// move rather than a rewrite.
type Pages struct{}

// text is one sentence in every language the platform speaks.
type text map[string]string

var (
	unknownHost = text{
		"en-US": "This address does not belong to any marketplace.",
		"pt-BR": "Este endereço não pertence a nenhum marketplace.",
	}
	inPreparation = text{
		"en-US": "This marketplace is being prepared and is not open yet.",
		"pt-BR": "Este marketplace está em preparação e ainda não abriu.",
	}
)

// UnknownHost answers a request whose host belongs to no marketplace.
//
// In both languages, and that is not indecision: the platform picks a language
// from the marketplace, the URL or the visitor's country, and this request has
// no marketplace to pick from. Guessing one would be a worse answer than
// showing both (docs/requirements.md, section 6).
func (Pages) UnknownHost(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	// Not indexable, whatever the deployment's setting: an address nobody
	// configured must never become a page in a search engine.
	w.Header().Set("X-Robots-Tag", "noindex, nofollow")
	w.WriteHeader(http.StatusNotFound)

	_, _ = w.Write([]byte(unknownHost["en-US"] + "\n" + unknownHost["pt-BR"] + "\n"))
}

// InPreparation answers a request for a marketplace that does not serve yet.
//
// It answers rather than refuses, on purpose: a marketplace leaves
// `in_preparation` only once its host is shown to answer, so a host that
// refused every request could never be activated (docs/design.md, decision 16).
//
// Here the language is known, because the marketplace is.
func (Pages) InPreparation(w http.ResponseWriter, _ *http.Request, marketplace *tenancy.Marketplace) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("X-Robots-Tag", "noindex, nofollow")
	w.WriteHeader(http.StatusOK)

	sentence, ok := inPreparation[marketplace.DefaultLanguage]
	if !ok {
		sentence = inPreparation["en-US"]
	}
	_, _ = w.Write([]byte(sentence + "\n"))
}
