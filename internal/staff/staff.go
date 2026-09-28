// Package staff is who works on the platform and what each may do in the
// console (docs/superpowers/specs/2026-09-26-f15-console-design.md).
//
// Staff are accounts without a marketplace (internal/identity, on a platform
// visit). This package holds what is theirs alone: the owner and the first
// run that creates them, and the answer to whether a staff member may do
// something. Everything it reads or writes belongs to the platform, so it
// runs in transactions that name no marketplace.
package staff

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/aleogr/marketplace/internal/platform/audit"
)

// Transactor opens a transaction that names no marketplace: the platform's
// (db.Pool.InTx).
type Transactor interface {
	InTx(ctx context.Context, fn func(pgx.Tx) error) error
}

// Auditor appends to the audit log ((*audit.Log).Append).
type Auditor interface {
	Append(ctx context.Context, tx pgx.Tx, entry audit.Entry) error
}

// Permission is something a staff member may be allowed to do in the
// console (D6). The registry of every permission, its labels and its
// synchronisation with the database arrive with the roles; these are the two
// the console needs before there are any.
type Permission string

const (
	// ConsoleView is signing in to the console and seeing its home.
	ConsoleView Permission = "console.view"
	// ConsoleVersion is reading the build identifier on /console/version.
	ConsoleVersion Permission = "console.version"
)

// Platform is the scope of what belongs to no marketplace — the console's
// home, its build identifier — as opposed to one marketplace's id.
const Platform = ""

// Authoriser answers whether a staff account may do something in a scope:
// in one marketplace, named by its id, or across the platform (Platform).
// The console asks it for every page it guards and every entry of its menu,
// and answers "not found" when it says no (D8).
//
// It is the seam between the owner and the roles: until roles exist the one
// implementation is Owners, which allows the owner everything, everywhere,
// and everybody else nothing; the roles' implementation replaces it without
// the console changing, answering from the roles assigned in the marketplace
// asked about or for the whole platform (D3, D6).
type Authoriser interface {
	Allows(ctx context.Context, account string, permission Permission, marketplace string) (bool, error)
}

// Owners allows the owner every permission and nobody else any. The owner
// holds every permission by their role, including those declared later
// (D7), which is why the permission is not looked up at all: the roles'
// Authoriser keeps this answer for the owner and adds the others'.
type Owners struct{ db Transactor }

// NewOwners returns the Authoriser of the first delivery of F15, in which the
// owner is the only staff member allowed anything.
func NewOwners(db Transactor) Owners { return Owners{db: db} }

// Allows reports whether account holds the owner role, whose scope is the
// whole platform and so every marketplace in it.
func (o Owners) Allows(ctx context.Context, account string, _ Permission, _ string) (bool, error) {
	var owner bool
	err := o.db.InTx(ctx, func(tx pgx.Tx) error {
		var err error
		owner, err = isOwner(ctx, tx, account)
		return err
	})
	return owner, err
}

// isOwner reports whether account holds the owner role.
func isOwner(ctx context.Context, tx pgx.Tx, account string) (bool, error) {
	var owner bool
	err := tx.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM user_role u JOIN role r ON r.id = u.role_id
		                WHERE u.account_id = $1 AND r.owner)`, account).Scan(&owner)
	return owner, err
}
