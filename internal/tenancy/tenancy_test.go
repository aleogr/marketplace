package tenancy_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aleogr/marketplace/internal/tenancy"
)

const platformHost = "marketplace.lab.aleogr.dev"

var (
	electronics = &tenancy.Marketplace{
		ID: "11111111-1111-1111-1111-111111111111", Slug: "marketplace1",
		Name: "Marketplace 1", MarketCode: "BR", RevenueModel: "commission",
		State: tenancy.Active, DefaultLanguage: "pt-BR",
	}
	unfinished = &tenancy.Marketplace{
		ID: "22222222-2222-2222-2222-222222222222", Slug: "marketplace2",
		Name: "Marketplace 2", MarketCode: "BR", RevenueModel: "paid_listings",
		State: tenancy.InPreparation, DefaultLanguage: "pt-BR",
	}
)

// loader serves a fixed host map and counts how often it is asked.
type loader struct {
	hosts map[string]*tenancy.Marketplace
	err   error
	calls int
}

func (l *loader) Hosts(context.Context) (map[string]*tenancy.Marketplace, error) {
	l.calls++
	if l.err != nil {
		return nil, l.err
	}
	return l.hosts, nil
}

func resolver() (*tenancy.Resolver, *loader) {
	source := &loader{hosts: map[string]*tenancy.Marketplace{
		"marketplace1." + platformHost: electronics,
		"marketplace2." + platformHost: unfinished,
	}}
	return tenancy.NewResolver(source, platformHost), source
}

func TestResolve(t *testing.T) {
	t.Parallel()

	for name, testCase := range map[string]struct {
		host            string
		wantKind        tenancy.Kind
		wantMarketplace *tenancy.Marketplace
	}{
		"a marketplace host": {
			host:            "marketplace1." + platformHost,
			wantKind:        tenancy.MarketplaceHost,
			wantMarketplace: electronics,
		},
		"the platform host": {
			host:     platformHost,
			wantKind: tenancy.PlatformHost,
		},
		"a host nobody configured": {
			host:     "someone-elses-domain.example",
			wantKind: tenancy.Unknown,
		},
		// A port is not part of a host's name. It reaches the process on every
		// local run, where the server listens on a port a test chose.
		"a host with a port": {
			host:            "marketplace1." + platformHost + ":8080",
			wantKind:        tenancy.MarketplaceHost,
			wantMarketplace: electronics,
		},
		"an uppercase host": {
			host:            "MARKETPLACE1." + platformHost,
			wantKind:        tenancy.MarketplaceHost,
			wantMarketplace: electronics,
		},
		// The root label's trailing dot is legal and rare, and it would
		// otherwise be a miss.
		"a fully qualified host": {
			host:            "marketplace1." + platformHost + ".",
			wantKind:        tenancy.MarketplaceHost,
			wantMarketplace: electronics,
		},
		// Resolution finds it; whether it serves is the middleware's decision,
		// because a marketplace that refused every request could never be
		// shown to answer and so could never be activated.
		"a marketplace still in preparation": {
			host:            "marketplace2." + platformHost,
			wantKind:        tenancy.MarketplaceHost,
			wantMarketplace: unfinished,
		},
		"no host at all": {
			host:     "",
			wantKind: tenancy.Unknown,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			subject, _ := resolver()
			got, err := subject.Resolve(t.Context(), testCase.host)
			if err != nil {
				t.Fatalf("Resolve(%q) = %v, want nil", testCase.host, err)
			}
			if got.Kind != testCase.wantKind {
				t.Errorf("Resolve(%q) kind = %v, want %v",
					testCase.host, got.Kind, testCase.wantKind)
			}
			if got.Marketplace != testCase.wantMarketplace {
				t.Errorf("Resolve(%q) marketplace = %v, want %v",
					testCase.host, got.Marketplace, testCase.wantMarketplace)
			}
		})
	}
}

func TestResolveReadsTheHostMapOnceUntilItIsInvalidated(t *testing.T) {
	t.Parallel()

	subject, source := resolver()
	host := "marketplace1." + platformHost

	for range 5 {
		if _, err := subject.Resolve(t.Context(), host); err != nil {
			t.Fatalf("Resolve() = %v, want nil", err)
		}
	}
	if source.calls != 1 {
		t.Errorf("the host map was read %d times, want 1: the cache is not holding",
			source.calls)
	}

	subject.Invalidate()
	if _, err := subject.Resolve(t.Context(), host); err != nil {
		t.Fatalf("Resolve() after Invalidate() = %v, want nil", err)
	}
	if source.calls != 2 {
		t.Errorf("the host map was read %d times, want 2: Invalidate() did not take effect",
			source.calls)
	}
}

func TestResolveReportsAFailureToReadTheHostMap(t *testing.T) {
	t.Parallel()

	// Answering "unknown host" when the map could not be read would be a lie
	// about the configuration, and picking a marketplace would be worse.
	source := &loader{err: errors.New("the database is unreachable")}
	subject := tenancy.NewResolver(source, platformHost)

	if _, err := subject.Resolve(t.Context(), "marketplace1."+platformHost); err == nil {
		t.Fatal("Resolve() hid a failure to read the host map, want an error")
	}
}

// pages records which answer the middleware asked for.
type pages struct{ unknown, preparing bool }

func (p *pages) UnknownHost(w http.ResponseWriter, _ *http.Request) {
	p.unknown = true
	w.WriteHeader(http.StatusNotFound)
}

func (p *pages) InPreparation(w http.ResponseWriter, _ *http.Request, _ *tenancy.Marketplace) {
	p.preparing = true
	w.WriteHeader(http.StatusOK)
}

func TestMiddleware(t *testing.T) {
	t.Parallel()

	const health = "/health"

	for name, testCase := range map[string]struct {
		host          string
		path          string
		wantServed    bool
		wantUnknown   bool
		wantPreparing bool
	}{
		"an active marketplace is served": {
			host:       "marketplace1." + platformHost,
			path:       "/",
			wantServed: true,
		},
		"the platform host is served": {
			host:       platformHost,
			path:       "/",
			wantServed: true,
		},
		"an unknown host is told so": {
			host:        "someone-elses-domain.example",
			path:        "/",
			wantUnknown: true,
		},
		"a marketplace in preparation gets its own answer": {
			host:          "marketplace2." + platformHost,
			path:          "/",
			wantPreparing: true,
		},
		// The health check reports on the process, not on a tenant, and it is
		// reached on the run.app address, which belongs to no marketplace and
		// never will. Behind host resolution it would fail every deployment.
		"the health check answers on any host": {
			host:       "marketplace-1084000440884.us-central1.run.app",
			path:       health,
			wantServed: true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			subject, _ := resolver()
			answers := &pages{}
			served := false

			handler := tenancy.Resolve(subject, answers, health)(
				http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
					served = true
				}))

			request := httptest.NewRequestWithContext(
				t.Context(), http.MethodGet, "http://"+testCase.host+testCase.path, nil)
			handler.ServeHTTP(httptest.NewRecorder(), request)

			if served != testCase.wantServed {
				t.Errorf("served = %v, want %v", served, testCase.wantServed)
			}
			if answers.unknown != testCase.wantUnknown {
				t.Errorf("unknown-host page = %v, want %v", answers.unknown, testCase.wantUnknown)
			}
			if answers.preparing != testCase.wantPreparing {
				t.Errorf("in-preparation page = %v, want %v", answers.preparing, testCase.wantPreparing)
			}
		})
	}
}

func TestMiddlewarePutsTheMarketplaceInTheContext(t *testing.T) {
	t.Parallel()

	subject, _ := resolver()
	var got tenancy.Resolution
	var found bool

	handler := tenancy.Resolve(subject, &pages{})(
		http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			got, found = tenancy.FromContext(r.Context())
		}))

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequestWithContext(
		t.Context(), http.MethodGet, "http://marketplace1."+platformHost+"/", nil))

	if !found {
		t.Fatal("the handler received no resolution in its context")
	}
	if got.Marketplace != electronics {
		t.Errorf("context carries %v, want %v", got.Marketplace, electronics)
	}
}

func TestFromContextReportsARequestThatSkippedTheMiddleware(t *testing.T) {
	t.Parallel()

	if _, found := tenancy.FromContext(t.Context()); found {
		t.Error("a context that never passed the middleware reported a resolution")
	}
}
