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
// Through a function, not from the tables: resolution runs before a
// marketplace is known — it is what decides which one — so it cannot be scoped
// by the answer it is looking for, and the tables are under row-level security
// (migrations/00004_row_level_security.sql). The function runs as the owning
// role and returns the routing map and nothing else.
//
// One query, not one per host: the whole map is small — a marketplace is
// created by a person, not by traffic — and reading it in one go is what lets
// resolution be a map lookup on the hot path.
const hostQuery = `SELECT host,
       marketplace_id,
       slug,
       name,
       market_code,
       revenue_model,
       state,
       detect_contact_data,
       reveal_contact,
       default_language,
       languages
FROM tenancy_host_map()
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
