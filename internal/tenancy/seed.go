package tenancy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/jackc/pgx/v5"
)

// Spec declares a marketplace an environment should have.
//
// Marketplaces are seeded from the environment rather than from a migration,
// and the reason is the hosts. A migration is versioned and every environment
// runs all of them, so a host written into one is created in every environment
// that ever exists — production would hold the lab's addresses. The environment
// is what knows its own hosts, Terraform already declares them beside the
// service, and this is the shape it passes them in (docs/roadmap.md, F5).
//
// The market itself stays in a migration, because a market is not
// environment-specific: Brazil is Brazil everywhere.
type Spec struct {
	Slug              string   `json:"slug"`
	Name              string   `json:"name"`
	Market            string   `json:"market"`
	RevenueModel      string   `json:"revenue_model"`
	State             string   `json:"state"`
	DefaultLanguage   string   `json:"default_language"`
	Languages         []string `json:"languages"`
	Hosts             []string `json:"hosts"`
	DetectContactData bool     `json:"detect_contact_data"`
	RevealContact     bool     `json:"reveal_contact"`
}

// ParseSpecs reads the marketplaces an environment declares, as JSON.
//
// An empty value declares none, which is what a local run and the end-to-end
// suite do. Anything that is present and unreadable is an error: a typo that
// silently seeded nothing would look exactly like a working deployment whose
// hosts all answer "unknown host".
func ParseSpecs(declared string) ([]Spec, error) {
	if declared == "" {
		return nil, nil
	}

	var specs []Spec
	if err := json.Unmarshal([]byte(declared), &specs); err != nil {
		return nil, fmt.Errorf("the declared marketplaces are not readable JSON: %w", err)
	}

	for i, spec := range specs {
		if err := spec.validate(); err != nil {
			return nil, fmt.Errorf("marketplace %d: %w", i+1, err)
		}
	}
	return specs, nil
}

func (s Spec) validate() error {
	for name, value := range map[string]string{
		"slug":          s.Slug,
		"name":          s.Name,
		"market":        s.Market,
		"revenue_model": s.RevenueModel,
	} {
		if value == "" {
			return fmt.Errorf("%s is missing", name)
		}
	}
	if len(s.Hosts) == 0 {
		return errors.New("no hosts: a marketplace nobody can reach is not a marketplace")
	}
	return nil
}

// Seed makes the database match what the environment declares.
//
// It is run by the migration job, after the migrations, and it is repeatable:
// every deployment runs it, and a deployment that changes nothing must leave
// the database unchanged. Hosts the environment no longer declares are removed,
// because the environment is what decides which hosts it answers on — a host
// left behind would keep resolving to a marketplace nobody is pointing DNS at.
func Seed(ctx context.Context, tx pgx.Tx, specs []Spec) ([]Applied, error) {
	applied := make([]Applied, 0, len(specs))
	for _, spec := range specs {
		one, err := seedOne(ctx, tx, spec)
		if err != nil {
			return nil, fmt.Errorf("cannot seed %q: %w", spec.Slug, err)
		}
		applied = append(applied, one)
	}
	return applied, nil
}

// Applied is what seeding did to one marketplace.
//
// It is returned rather than logged, because the caller is what writes the
// audit record of it: a marketplace appearing or changing is a parameter change
// (docs/requirements.md, section 21), and this package has no business knowing
// how those are recorded.
type Applied struct {
	Spec Spec
	ID   string
	// Created is true when this deployment is the one that brought the
	// marketplace into existence.
	Created bool
}

func seedOne(ctx context.Context, tx pgx.Tx, spec Spec) (Applied, error) {
	state := spec.State
	if state == "" {
		state = string(Active)
	}
	language := spec.DefaultLanguage
	if language == "" {
		language = "en-US"
	}

	var id string
	var created bool
	err := tx.QueryRow(ctx, `
		INSERT INTO marketplace (
		    slug, name, market_code, revenue_model, state,
		    detect_contact_data, reveal_contact, default_language
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (slug) DO UPDATE SET
		    name                = EXCLUDED.name,
		    market_code         = EXCLUDED.market_code,
		    revenue_model       = EXCLUDED.revenue_model,
		    state               = EXCLUDED.state,
		    detect_contact_data = EXCLUDED.detect_contact_data,
		    reveal_contact      = EXCLUDED.reveal_contact,
		    default_language    = EXCLUDED.default_language
		-- xmax = 0 is true of a row this statement inserted and false of one
		-- it updated, which is how an upsert says which of the two it did.
		RETURNING id::text, (xmax = 0)
	`, spec.Slug, spec.Name, spec.Market, spec.RevenueModel, state,
		spec.DetectContactData, spec.RevealContact, language).Scan(&id, &created)
	if err != nil {
		return Applied{}, fmt.Errorf("cannot write the marketplace: %w", err)
	}

	languages := spec.Languages
	if !slices.Contains(languages, language) {
		// A marketplace whose default language is not among its languages
		// would have a default nobody can choose.
		languages = append(languages, language)
	}

	if _, err := tx.Exec(ctx, `
		DELETE FROM marketplace_language
		WHERE marketplace_id = $1 AND language <> ALL($2::text[])
	`, id, languages); err != nil {
		return Applied{}, fmt.Errorf("cannot remove languages: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO marketplace_language (marketplace_id, language)
		SELECT $1, language FROM unnest($2::text[]) AS language
		ON CONFLICT DO NOTHING
	`, id, languages); err != nil {
		return Applied{}, fmt.Errorf("cannot add languages: %w", err)
	}

	hosts := make([]string, 0, len(spec.Hosts))
	for _, host := range spec.Hosts {
		hosts = append(hosts, Normalise(host))
	}

	if _, err := tx.Exec(ctx, `
		DELETE FROM marketplace_host
		WHERE marketplace_id = $1 AND host <> ALL($2::text[])
	`, id, hosts); err != nil {
		return Applied{}, fmt.Errorf("cannot remove hosts: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO marketplace_host (host, marketplace_id)
		SELECT host, $1 FROM unnest($2::text[]) AS host
		ON CONFLICT (host) DO UPDATE SET marketplace_id = EXCLUDED.marketplace_id
	`, id, hosts); err != nil {
		return Applied{}, fmt.Errorf("cannot add hosts: %w", err)
	}

	return Applied{Spec: spec, ID: id, Created: created}, nil
}
