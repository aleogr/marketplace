//go:build integration

package identity

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

// identityRows are the tables whose rows belong to a marketplace or, for
// staff, to the platform (F15 spec, D4), in an order that satisfies their
// references, each with the statement that writes one row for an account —
// $1 the account, $2 the scope ("" for the platform), $3 a session's token
// hash — and the column that names the account a row is about.
var identityRows = []struct{ table, column, insert string }{
	{"credential", "account_id", `INSERT INTO credential (account_id, marketplace_id, kind, secret)
		VALUES ($1, nullif($2, '')::uuid, 'password', encode($3, 'hex'))`},
	{"email_verification", "account_id", `INSERT INTO email_verification (token_hash, account_id, marketplace_id, expires_at)
		VALUES ($3 || '-verify'::bytea, $1, nullif($2, '')::uuid, now() + interval '1 day')`},
	{"session", "account_id", `INSERT INTO session (account_id, marketplace_id, token_hash, ip, user_agent)
		VALUES ($1, nullif($2, '')::uuid, $3, '203.0.113.7', 'test')`},
	{"second_factor", "account_id", `INSERT INTO second_factor (account_id, marketplace_id, kind, label, secret)
		VALUES ($1, nullif($2, '')::uuid, 'totp', 'phone', $3)`},
	{"recovery_code", "account_id", `INSERT INTO recovery_code (account_id, marketplace_id, code_hash)
		VALUES ($1, nullif($2, '')::uuid, $3)`},
	{"factor_enrolment", "account_id", `INSERT INTO factor_enrolment (session_id, account_id, marketplace_id, kind, secret, expires_at)
		VALUES ((SELECT id FROM session WHERE token_hash = $3), $1, nullif($2, '')::uuid, 'totp', 'sealed',
		        now() + interval '15 minutes')`},
	{"sign_in_challenge", "account_id", `INSERT INTO sign_in_challenge (token_hash, account_id, marketplace_id, expires_at)
		VALUES ($3 || '-challenge'::bytea, $1, nullif($2, '')::uuid, now() + interval '5 minutes')`},
	{"email_code", "account_id", `INSERT INTO email_code (account_id, marketplace_id, purpose, code_hash, expires_at)
		VALUES ($1, nullif($2, '')::uuid, 'signin', $3, now() + interval '10 minutes')`},
	{"second_factor_failure", "account_id", `INSERT INTO second_factor_failure (account_id, marketplace_id, failures, updated_at)
		VALUES ($1, nullif($2, '')::uuid, length($3), now())`},
}

// everyIdentityRow writes an account of scope — a marketplace's id, or "" for
// the platform, whose accounts are staff — and one row of every other
// identity table for it, in a transaction of that scope, as the application
// role. It returns the account's id.
func everyIdentityRow(t *testing.T, db serving, scope, email string) string {
	t.Helper()
	kind := "buyer"
	if scope == "" {
		kind = "staff"
	}
	var account string
	if err := db.InTxFor(t.Context(), scope, func(tx pgx.Tx) error {
		if err := tx.QueryRow(t.Context(), `
			INSERT INTO account (marketplace_id, kind, email, email_normalised, name)
			VALUES (nullif($1, '')::uuid, $2, $3, $3, 'Someone') RETURNING id::text`,
			scope, kind, email).Scan(&account); err != nil {
			return fmt.Errorf("account: %w", err)
		}
		for _, row := range identityRows {
			if _, err := tx.Exec(t.Context(), row.insert, account, scope, []byte(email)); err != nil {
				return fmt.Errorf("%s: %w", row.table, err)
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("writing the identity rows of scope %q: %v", scope, err)
	}
	return account
}

// visible counts the rows of table about account that a transaction of
// scope sees, and deletes, which it must not be able to do either for rows
// it cannot see: it reports how many rows it saw and how many it deleted,
// and rolls the deletion back.
func visible(t *testing.T, db serving, scope, table, column, account string) (seen, deleted int64) {
	t.Helper()
	rolledBack := errors.New("roll back")
	err := db.InTxFor(t.Context(), scope, func(tx pgx.Tx) error {
		if err := tx.QueryRow(t.Context(),
			`SELECT count(*) FROM `+table+` WHERE `+column+` = $1`, account).Scan(&seen); err != nil {
			return err
		}
		tag, err := tx.Exec(t.Context(), `DELETE FROM `+table+` WHERE `+column+` = $1`, account)
		if err != nil {
			return err
		}
		deleted = tag.RowsAffected()
		return rolledBack
	})
	if !errors.Is(err, rolledBack) {
		t.Fatalf("reading %s from scope %q: %v", table, scope, err)
	}
	return seen, deleted
}

// Every identity table answers a marketplace's transaction with that
// marketplace's rows and a transaction that names no marketplace with the
// platform's, which are staff's, and neither with the other's: a marketplace
// never sees or deletes a staff member's rows, and the platform never a
// buyer's (F15 spec, D4 and Risks).
func TestIdentityRowsBelongToAMarketplaceOrToThePlatform(t *testing.T) {
	db, one, two := twoMarketplaces(t)
	buyer := everyIdentityRow(t, db, one, unique(t, "buyer")+"@example.test")
	staff := everyIdentityRow(t, db, "", unique(t, "staff")+"@example.test")

	tables := append([]struct{ table, column, insert string }{{"account", "id", ""}}, identityRows...)
	for _, row := range tables {
		for _, tc := range []struct {
			name, scope, account string
			want                 int64
		}{
			{"the marketplace sees its buyer", one, buyer, 1},
			{"the platform sees its staff", "", staff, 1},
			{"the marketplace never sees staff", one, staff, 0},
			{"another marketplace never sees staff", two, staff, 0},
			{"the platform never sees a buyer", "", buyer, 0},
			{"another marketplace never sees the buyer", two, buyer, 0},
		} {
			t.Run(row.table+"/"+tc.name, func(t *testing.T) {
				seen, deleted := visible(t, db, tc.scope, row.table, row.column, tc.account)
				if seen != tc.want || deleted != tc.want {
					t.Errorf("saw %d and deleted %d rows, want %d", seen, deleted, tc.want)
				}
			})
		}
	}
}

// Nor can one scope write a row into the other: the policies check what is
// written as they check what is read.
func TestIdentityRowsCannotBeWrittenIntoTheOtherScope(t *testing.T) {
	db, one, _ := twoMarketplaces(t)
	for _, tc := range []struct{ name, scope, marketplace, kind string }{
		{"a marketplace writing a staff account", one, "", "staff"},
		{"the platform writing a buyer account", "", one, "buyer"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := db.InTxFor(t.Context(), tc.scope, func(tx pgx.Tx) error {
				_, err := tx.Exec(t.Context(), `
					INSERT INTO account (marketplace_id, kind, email, email_normalised, name)
					VALUES (nullif($1, '')::uuid, $2, $3, $3, 'Someone')`,
					tc.marketplace, tc.kind, unique(t, "w")+"@example.test")
				return err
			})
			if err == nil {
				t.Fatal("the row was written into the other scope")
			}
		})
	}
}

// The sweep names its scope, so that the index on session.marketplace_id
// narrows it — the policy's IS NOT DISTINCT FROM is not something an index
// answers — and the platform's sweep removes staff's stale sessions and never
// a marketplace's, as a marketplace's never removes staff's.
func TestTheSweepRemovesOnlyItsOwnScopesSessions(t *testing.T) {
	db, one, _ := twoMarketplaces(t)
	buyer := everyIdentityRow(t, db, one, unique(t, "buyer")+"@example.test")
	staff := everyIdentityRow(t, db, "", unique(t, "staff")+"@example.test")
	now := time.Now().UTC()
	for scope, account := range map[string]string{one: buyer, "": staff} {
		if err := db.InTxFor(t.Context(), scope, func(tx pgx.Tx) error {
			_, err := tx.Exec(t.Context(), `UPDATE session SET created_at = $2 WHERE account_id = $1`,
				account, now.Add(-SessionLifetime-time.Hour))
			return err
		}); err != nil {
			t.Fatal(err)
		}
	}
	sweep := func(scope string) {
		t.Helper()
		if err := db.InTxFor(t.Context(), scope, func(tx pgx.Tx) error {
			_, err := sweepSessions(t.Context(), tx, scope, now)
			return err
		}); err != nil {
			t.Fatal(err)
		}
	}

	sweep("")
	if seen, _ := visible(t, db, "", "session", "account_id", staff); seen != 0 {
		t.Error("the platform's sweep left a stale staff session")
	}
	if seen, _ := visible(t, db, one, "session", "account_id", buyer); seen != 1 {
		t.Error("the platform's sweep removed a marketplace's session")
	}
	sweep(one)
	if seen, _ := visible(t, db, one, "session", "account_id", buyer); seen != 0 {
		t.Error("the marketplace's sweep left its stale session")
	}
}

// accountByEmail's lookup finds its account through account_by_email: the
// unique index leads with marketplace_id, which the policy's
// IS NOT DISTINCT FROM (this migration) is not an index condition for, so
// without a second index every sign-in would scan every account
// (migrations/00016_platform_identity_rows.sql).
func TestAccountByEmailUsesTheIndex(t *testing.T) {
	db, one, _ := twoMarketplaces(t)
	email := unique(t, "lookup") + "@example.test"
	everyIdentityRow(t, db, one, email)

	var plan string
	if err := db.InTxFor(t.Context(), one, func(tx pgx.Tx) error {
		// The table holds only a handful of rows in this test, which the
		// planner may cost a sequential scan for regardless of the index;
		// enable_seqscan = off makes it price the index instead, the way it
		// already would at the row counts a live marketplace holds.
		if _, err := tx.Exec(t.Context(), "SET LOCAL enable_seqscan = off"); err != nil {
			return err
		}
		return tx.QueryRow(t.Context(),
			`EXPLAIN (FORMAT JSON) SELECT id FROM account WHERE email_normalised = $1`, email).Scan(&plan)
	}); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(plan, "account_by_email") {
		t.Fatalf("the plan does not use account_by_email: %s", plan)
	}
}
