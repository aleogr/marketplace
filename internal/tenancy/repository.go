package tenancy

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// Queryer is the part of the connection pool this repository uses. Taking the
// two methods rather than the pool keeps the package testable without a
// database and free of a dependency on how the pool is built.
type Queryer interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// Repository reads the host map from PostgreSQL.
type Repository struct {
	queryer Queryer
}

// NewRepository returns a repository reading through queryer.
func NewRepository(queryer Queryer) *Repository {
	return &Repository{queryer: queryer}
}

// hostQuery reads every host with the marketplace behind it.
//
// One query, not one per host: the whole map is small — a marketplace is
// created by a person, not by traffic — and reading it in one go is what lets
// resolution be a map lookup on the hot path.
const hostQuery = `
SELECT h.host,
       m.id::text,
       m.slug,
       m.name,
       m.market_code,
       m.revenue_model,
       m.state,
       m.detect_contact_data,
       m.reveal_contact,
       m.default_language,
       COALESCE(
           ARRAY(
               SELECT l.language
               FROM marketplace_language l
               WHERE l.marketplace_id = m.id
               ORDER BY l.language
           ),
           ARRAY[]::text[]
       ) AS languages
FROM marketplace_host h
JOIN marketplace m ON m.id = h.marketplace_id
`

// Hosts returns every configured host and the marketplace it belongs to.
func (r *Repository) Hosts(ctx context.Context) (map[string]*Marketplace, error) {
	rows, err := r.queryer.Query(ctx, hostQuery)
	if err != nil {
		return nil, fmt.Errorf("cannot read the host map: %w", err)
	}
	defer rows.Close()

	hosts := map[string]*Marketplace{}
	// Two hosts of one marketplace share one value, so that identity means
	// what it looks like it means to everything downstream.
	byID := map[string]*Marketplace{}

	for rows.Next() {
		var host string
		var marketplace Marketplace

		if err := rows.Scan(
			&host,
			&marketplace.ID,
			&marketplace.Slug,
			&marketplace.Name,
			&marketplace.MarketCode,
			&marketplace.RevenueModel,
			&marketplace.State,
			&marketplace.DetectContactData,
			&marketplace.RevealContact,
			&marketplace.DefaultLanguage,
			&marketplace.Languages,
		); err != nil {
			return nil, fmt.Errorf("cannot read a host: %w", err)
		}

		shared, seen := byID[marketplace.ID]
		if !seen {
			shared = &marketplace
			byID[marketplace.ID] = shared
		}
		hosts[Normalise(host)] = shared
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("cannot read the host map: %w", err)
	}
	return hosts, nil
}
