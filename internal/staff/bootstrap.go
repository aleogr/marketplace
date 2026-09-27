package staff

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/aleogr/marketplace/internal/identity"
	"github.com/aleogr/marketplace/internal/platform/audit"
)

var (
	// ErrSetupClosed is a first run that is not there: the owner exists, or
	// this deployment was given no token. The console answers it as "not
	// found" (D2, D8).
	ErrSetupClosed = errors.New("staff: there is no first run to serve")
	// ErrTokenWrong is a token that is not the first run's.
	ErrTokenWrong = errors.New("staff: the bootstrap token is wrong")
)

// Bootstrap is the first run: while no owner exists, whoever presents the
// token Terraform generated creates the owner (D2). The token is read from
// the configuration and never written anywhere; only its SHA-256 is kept,
// once it is used.
type Bootstrap struct {
	db       Transactor
	identity *identity.Service
	audit    Auditor
	// hash is the token's SHA-256; set is whether there is a token at all.
	hash [sha256.Size]byte
	set  bool
	now  func() time.Time
}

// NewBootstrap returns the first run for token, or none when token is "".
func NewBootstrap(db Transactor, service *identity.Service, auditor Auditor, token string) *Bootstrap {
	return &Bootstrap{db: db, identity: service, audit: auditor, hash: sha256.Sum256([]byte(token)),
		set: token != "", now: func() time.Time { return time.Now().UTC() }}
}

// Open reports whether the first run is served: there is a token, and no
// owner yet.
func (b *Bootstrap) Open(ctx context.Context) (bool, error) {
	if !b.set {
		return false, nil
	}
	var done bool
	err := b.db.InTx(ctx, func(tx pgx.Tx) error {
		var err error
		done, err = ownerExists(ctx, tx)
		return err
	})
	return !done, err
}

// ownerExists reports whether the first run happened, or an owner exists by
// any other road — the manual recovery of docs/infrastructure.md, for one.
func ownerExists(ctx context.Context, tx pgx.Tx) (bool, error) {
	var exists bool
	err := tx.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM bootstrap)
		    OR EXISTS (SELECT 1 FROM user_role u JOIN role r ON r.id = u.role_id WHERE r.owner)`).Scan(&exists)
	return exists, err
}

// Setup creates the owner, when token is the first run's and no owner
// exists: a staff account with name, email and password (F13's rules, the
// breach check included), the owner role assigned to it for the whole
// platform, and the token recorded as used, all in one transaction. It
// signs the owner in and returns the session's token; the console then asks
// for a second factor before anything else (D4).
//
// The token is compared by its SHA-256, in constant time, so the comparison
// takes as long whatever the token's first wrong character. Two first runs
// at once make one owner: the second's claim of the one bootstrap row waits
// for the first's and finds the setup closed.
func (b *Bootstrap) Setup(ctx context.Context, v identity.Visit, token, name, email, password string) (string, error) {
	open, err := b.Open(ctx)
	if err != nil {
		return "", err
	}
	if !open {
		return "", ErrSetupClosed
	}
	given := sha256.Sum256([]byte(token))
	if subtle.ConstantTimeCompare(given[:], b.hash[:]) != 1 {
		return "", ErrTokenWrong
	}
	session, err := b.identity.CreateStaff(ctx, v, name, email, password, func(ctx context.Context, tx pgx.Tx, account string) error {
		return b.claim(ctx, tx, v, account)
	})
	if errors.Is(err, identity.ErrAddressTaken) {
		// Two first runs at once with the same e-mail race at the account, not
		// at the bootstrap row: the loser's insert waits on the unique index
		// and then finds it taken, by the winner's own account rather than by
		// someone else's. Once the winner has made an owner, the loser's
		// address being taken is the setup closing under it, not a real
		// conflict, so it gets the same answer every other latecomer does.
		if open, openErr := b.Open(ctx); openErr == nil && !open {
			return "", ErrSetupClosed
		}
	}
	return session, err
}

// claim records the first run as done by account and makes account the
// owner, or reports ErrSetupClosed when another first run got there first.
func (b *Bootstrap) claim(ctx context.Context, tx pgx.Tx, v identity.Visit, account string) error {
	tag, err := tx.Exec(ctx, `
		INSERT INTO bootstrap (token_hash, used_at, owner_id) VALUES ($1, $2, $3)
		ON CONFLICT (singleton) DO NOTHING`, b.hash[:], b.now(), account)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrSetupClosed
	}
	var role string
	if err := tx.QueryRow(ctx,
		`INSERT INTO role (name, owner) VALUES ('owner', true) RETURNING id::text`).Scan(&role); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO user_role (account_id, role_id, marketplace_id, assigned_by, assigned_at)
		VALUES ($1, $2, NULL, NULL, $3)`, account, role, b.now()); err != nil {
		return err
	}
	after, err := json.Marshal(map[string]string{"role": role, "scope": "platform", "user_agent": v.UserAgent})
	if err != nil {
		return err
	}
	return b.audit.Append(ctx, tx, audit.Entry{
		Actor:   audit.Actor{ID: account, Kind: audit.Staff},
		Action:  "staff.owner_created",
		Subject: audit.Subject{Kind: "account", ID: account, Person: account},
		From:    v.IP,
		After:   after,
	})
}
