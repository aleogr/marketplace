package tenancy

import (
	"context"
	"net/http"
)

// contextKey is unexported so that nothing outside this package can put a
// marketplace into a request's context, or take one out by guessing the key.
type contextKey struct{}

// WithResolution returns a context carrying what the request's host resolved to.
func WithResolution(ctx context.Context, resolution Resolution) context.Context {
	return context.WithValue(ctx, contextKey{}, resolution)
}

// FromContext returns what the request's host resolved to.
//
// The second result is false for a request that never passed through the
// middleware, which is a programming error rather than a runtime condition: a
// handler that reads a marketplace must be mounted behind it.
func FromContext(ctx context.Context) (Resolution, bool) {
	resolution, ok := ctx.Value(contextKey{}).(Resolution)
	return resolution, ok
}

// Unresolved is asked to answer a request whose host belongs to no marketplace,
// and a request whose marketplace is not serving yet.
//
// It is an argument rather than a fixed page because what those requests should
// see is the user interface's decision, and this package has no templates.
type Unresolved interface {
	UnknownHost(w http.ResponseWriter, r *http.Request)
	InPreparation(w http.ResponseWriter, r *http.Request, marketplace *Marketplace)
}

// Resolve is the first middleware of the request pipeline.
//
// `bypass` names the paths that answer before any marketplace is known. The
// health check is one: it reports on the process, not on a tenant, and it is
// reached on the platform's own `run.app` address, which belongs to no
// marketplace and never will. A health check behind host resolution would fail
// every deployment (docs/roadmap.md, F5).
func Resolve(resolver *Resolver, unresolved Unresolved, bypass ...string) func(http.Handler) http.Handler {
	skip := make(map[string]bool, len(bypass))
	for _, path := range bypass {
		skip[path] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if skip[r.URL.Path] {
				next.ServeHTTP(w, r)
				return
			}

			resolution, err := resolver.Resolve(r.Context(), r.Host)
			if err != nil {
				// The host map could not be read. Answering as though the host
				// were unknown would be a lie about the configuration, and
				// picking a marketplace would be worse.
				http.Error(w, "the marketplace could not be determined",
					http.StatusServiceUnavailable)
				return
			}

			switch resolution.Kind {
			case Unknown:
				unresolved.UnknownHost(w, r)
				return
			case MarketplaceHost:
				if resolution.Marketplace.State != Active {
					// Answering rather than refusing is deliberate: a
					// marketplace leaves `in_preparation` only once its host is
					// shown to answer, so a host that refused every request
					// could never be activated.
					unresolved.InPreparation(w, r, resolution.Marketplace)
					return
				}
			case PlatformHost:
			}

			next.ServeHTTP(w, r.WithContext(
				WithResolution(r.Context(), resolution)))
		})
	}
}
