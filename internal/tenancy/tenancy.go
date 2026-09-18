// Package tenancy answers which marketplace a request belongs to.
//
// The host of the incoming request decides, from day one
// (docs/requirements.md, section 7). Resolution is the first thing the request
// pipeline does, because everything after it — language, session, tenant
// context, every query — is scoped by the answer (docs/design.md, section 2.3).
//
// There is deliberately no default marketplace. A host nobody configured
// resolves to nothing and is told so; falling back to "the first one" would
// serve one marketplace's data under another's name, which is the failure this
// whole package exists to make impossible.
package tenancy

import (
	"context"
	"errors"
	"net"
	"strings"
	"sync"
	"time"
)

// State is where a marketplace is in its life.
type State string

const (
	// InPreparation is a marketplace that exists but does not serve. It leaves
	// this state only once its host answers and its e-mail domain verifies
	// (docs/design.md, decision 16) — which is why a host in this state must
	// answer *something*: a marketplace whose host refused every request could
	// never prove the host works and could never be activated.
	InPreparation State = "in_preparation"
	// Active is a marketplace that serves.
	Active State = "active"
)

// Marketplace is one marketplace of the platform.
type Marketplace struct {
	ID              string
	Slug            string
	Name            string
	MarketCode      string
	RevenueModel    string
	State           State
	DefaultLanguage string
	Languages       []string

	// DetectContactData masks telephone numbers, addresses and links in text.
	// On for goods marketplaces, off for vehicles, where the revenue comes
	// from listings rather than from the transaction and masking would be both
	// porous and pointless (docs/design.md, decision 2).
	DetectContactData bool
	// RevealContact lets a store publish its telephone on a listing.
	RevealContact bool
}

// Kind is what a host turned out to be.
type Kind int

const (
	// Unknown is a host nobody configured.
	Unknown Kind = iota
	// PlatformHost is the platform's own host: the administrative console and
	// everything that belongs to no marketplace.
	PlatformHost
	// MarketplaceHost is a host a marketplace answers on.
	MarketplaceHost
)

// Resolution is what a host resolved to.
type Resolution struct {
	Kind Kind
	// Marketplace is set only when Kind is MarketplaceHost.
	Marketplace *Marketplace
}

// Loader reads the host map. The repository implements it; tests replace it.
type Loader interface {
	Hosts(ctx context.Context) (map[string]*Marketplace, error)
}

// defaultTTL is how long the host map is trusted without asking again.
//
// Short, because a marketplace is created by a person who then wants to see it
// answer, and a long cache would make that wait look like a bug. Not zero,
// because every request would otherwise query the database before doing
// anything else. Creating a marketplace invalidates the cache explicitly; the
// timer is only there for the instance that did not do the creating.
const defaultTTL = 30 * time.Second

// Resolver answers hosts, holding the map briefly between reads.
type Resolver struct {
	loader       Loader
	platformHost string
	ttl          time.Duration
	now          func() time.Time

	mu       sync.RWMutex
	hosts    map[string]*Marketplace
	loadedAt time.Time
}

// NewResolver returns a resolver for the given platform host.
//
// The platform's own host is configuration rather than data: it belongs to the
// deployment, is declared in Terraform alongside the service, and exists before
// any marketplace does.
func NewResolver(loader Loader, platformHost string) *Resolver {
	return &Resolver{
		loader:       loader,
		platformHost: Normalise(platformHost),
		ttl:          defaultTTL,
		now:          time.Now,
	}
}

// Resolve reports what host is.
func (r *Resolver) Resolve(ctx context.Context, host string) (Resolution, error) {
	name := Normalise(host)
	if name == "" {
		return Resolution{Kind: Unknown}, nil
	}

	if r.platformHost != "" && name == r.platformHost {
		return Resolution{Kind: PlatformHost}, nil
	}

	hosts, err := r.load(ctx)
	if err != nil {
		return Resolution{}, err
	}

	if marketplace, ok := hosts[name]; ok {
		return Resolution{Kind: MarketplaceHost, Marketplace: marketplace}, nil
	}
	return Resolution{Kind: Unknown}, nil
}

// Invalidate forgets the host map, so that the next resolution reads it again.
//
// Called by whatever changes the map. The cache's timer is the fallback for
// other instances; this is the path that makes a new marketplace answer at
// once on the instance that created it.
func (r *Resolver) Invalidate() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.hosts = nil
	r.loadedAt = time.Time{}
}

func (r *Resolver) load(ctx context.Context) (map[string]*Marketplace, error) {
	r.mu.RLock()
	if r.hosts != nil && r.now().Sub(r.loadedAt) < r.ttl {
		hosts := r.hosts
		r.mu.RUnlock()
		return hosts, nil
	}
	r.mu.RUnlock()

	hosts, err := r.loader.Hosts(ctx)
	if err != nil {
		return nil, err
	}
	if hosts == nil {
		// A repository that returns no rows returns an empty map, not nil;
		// nil would be read as "never loaded" and queried again every request.
		hosts = map[string]*Marketplace{}
	}

	r.mu.Lock()
	r.hosts = hosts
	r.loadedAt = r.now()
	r.mu.Unlock()

	return hosts, nil
}

// ErrNoHost is returned when a request carries no host at all.
var ErrNoHost = errors.New("the request carries no host")

// Normalise reduces a host to the form the database stores.
//
// A host is case-insensitive, may carry a port that is not part of its name,
// and may end in the root label's dot. Normalising before the lookup is what
// makes `MARKETPLACE1.example:8080`, `marketplace1.example.` and
// `marketplace1.example` one host rather than three misses.
func Normalise(host string) string {
	host = strings.TrimSpace(host)
	if host == "" {
		return ""
	}

	// SplitHostPort fails when there is no port, which is the common case and
	// not an error; the host is then already what was given.
	if withoutPort, _, err := net.SplitHostPort(host); err == nil {
		host = withoutPort
	}

	host = strings.TrimSuffix(host, ".")
	return strings.ToLower(host)
}
