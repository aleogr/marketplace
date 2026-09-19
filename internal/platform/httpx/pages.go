package httpx

import (
	"net/http"
	"time"

	"github.com/aleogr/marketplace/internal/platform/ratelimit"
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
	forbiddenRequest = text{
		"en-US": "This request could not be verified. Reload the page and try again.",
		"pt-BR": "Não foi possível verificar esta solicitação. Recarregue a página e tente novamente.",
	}
	tooManyRequests = text{
		"en-US": "Too many requests. Wait a moment and try again.",
		"pt-BR": "Solicitações demais. Espere um momento e tente novamente.",
	}
)

// bothLanguages writes a short message in every language the platform speaks.
//
// These are the answers given before a marketplace is known, or instead of
// what the visitor asked for, so there is no marketplace to take a language
// from — and guessing one would be a worse answer than showing both
// (docs/requirements.md, section 6).
func bothLanguages(w http.ResponseWriter, status int, message text) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(message["en-US"] + "\n" + message["pt-BR"] + "\n"))
}

// forbidden answers a request that could not prove where it came from.
func forbidden(w http.ResponseWriter) {
	bothLanguages(w, http.StatusForbidden, forbiddenRequest)
}

// Refused answers a request that has come too often.
//
// It is exported because the rate limiter mounts it, and the limiter knows
// nothing about pages (internal/platform/ratelimit).
func Refused(w http.ResponseWriter, _ *http.Request, retryAfter time.Duration) {
	ratelimit.RetryAfter(w, retryAfter)
	bothLanguages(w, http.StatusTooManyRequests, tooManyRequests)
}

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
