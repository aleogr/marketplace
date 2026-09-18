// Package db opens and holds the connection pool the process talks to
// PostgreSQL through.
//
// It knows two routes to the same database. A direct connection string is what
// a cluster in a session and a service container in CI offer. A Cloud SQL
// connection name goes through Google's connector, which authenticates the
// process's own service account with IAM and encrypts the connection with
// certificates the Cloud SQL API issues — so a deployment holds no database
// password at all (docs/design.md, section 3).
package db

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"cloud.google.com/go/cloudsqlconn"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/aleogr/marketplace/internal/platform/config"
)

// maxConns bounds how many connections one process opens.
//
// It is deliberately small. The lab runs on the smallest machine Cloud SQL
// offers, whose PostgreSQL allows a few dozen connections in total
// (docs/infrastructure.md), and that budget is shared by every Cloud Run
// instance and by the migration job. A pool that sizes itself to the host's
// CPUs would exhaust it as soon as the service scaled.
const maxConns = 4

// pingTimeout bounds a health check's query, so that an unreachable database
// answers the check rather than holding the request open until something else
// gives up first.
const pingTimeout = 2 * time.Second

// Pool is the process's connection pool.
type Pool struct {
	*pgxpool.Pool

	// closeDialer releases the Cloud SQL connector, when one is in use.
	closeDialer func() error
}

// Open connects to the database described by settings.
//
// It returns an error when no database is declared: whether the process runs
// without one is the caller's decision, made from the configuration, not a
// thing this package guesses.
func Open(ctx context.Context, settings config.Database) (*Pool, error) {
	if !settings.Configured() {
		return nil, errors.New("no database is configured")
	}

	poolConfig, err := pgxpool.ParseConfig(dsn(settings))
	if err != nil {
		return nil, fmt.Errorf("cannot read the database settings: %w", err)
	}
	poolConfig.MaxConns = maxConns

	pool := &Pool{}

	if settings.Instance != "" {
		var options []cloudsqlconn.Option
		if settings.Password == "" {
			// No password means this process authenticates as itself. The
			// connector exchanges the service account's token for the login,
			// which is why nothing here holds a credential.
			options = append(options, cloudsqlconn.WithIAMAuthN())
		}

		dialer, err := cloudsqlconn.NewDialer(ctx, options...)
		if err != nil {
			return nil, fmt.Errorf("cannot start the Cloud SQL connector: %w", err)
		}
		pool.closeDialer = dialer.Close

		// The connector dials the instance by name and brings its own
		// encryption, which is why the connection string says sslmode=disable:
		// it disables a *second* TLS handshake inside the first, not the
		// encryption.
		poolConfig.ConnConfig.DialFunc = func(ctx context.Context, _, _ string) (net.Conn, error) {
			return dialer.Dial(ctx, settings.Instance)
		}
	}

	pool.Pool, err = pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		pool.closeConnector()
		return nil, fmt.Errorf("cannot open the connection pool: %w", err)
	}

	return pool, nil
}

// dsn builds the connection string for the route settings describe.
func dsn(settings config.Database) string {
	if settings.URL != "" {
		return settings.URL
	}

	dsn := fmt.Sprintf("dbname=%s user=%s sslmode=disable",
		settings.Name, settings.User)
	if settings.Password != "" {
		dsn += " password=" + settings.Password
	}
	return dsn
}

// Ping reports whether the database answers.
func (p *Pool) Ping(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, pingTimeout)
	defer cancel()
	return p.Pool.Ping(ctx)
}

// InTx runs fn inside a transaction, committing when it returns nil and
// rolling back otherwise.
//
// Every write goes through here rather than through the pool directly, because
// this is where the tenant session variable is set once row-level security
// arrives (docs/design.md, section 2.6): a write that bypassed it would be a
// write with no tenant.
func (p *Pool) InTx(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := p.Begin(ctx)
	if err != nil {
		return fmt.Errorf("cannot begin a transaction: %w", err)
	}

	// A rollback after a successful commit is what pgx answers with
	// ErrTxClosed, and that is not a failure worth reporting.
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()

	if err := fn(tx); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("cannot commit: %w", err)
	}
	return nil
}

// Close releases the pool and the connector behind it.
func (p *Pool) Close() {
	if p.Pool != nil {
		p.Pool.Close()
	}
	p.closeConnector()
}

func (p *Pool) closeConnector() {
	if p.closeDialer != nil {
		_ = p.closeDialer()
		p.closeDialer = nil
	}
}
