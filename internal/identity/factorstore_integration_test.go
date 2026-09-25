//go:build integration

package identity

import (
	"bytes"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

// anAccount inserts an unconfirmed account in marketplace and returns its id.
func anAccount(t *testing.T, db serving, marketplace, email string) string {
	t.Helper()
	var id string
	if err := db.InTxFor(t.Context(), marketplace, func(tx pgx.Tx) error {
		var err error
		id, err = insertAccount(t.Context(), tx, marketplace, email, email, "Reader")
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return id
}

// A marketplace sees its own accounts' second factors and nothing of
// another's, like every other tenant table.
func TestSecondFactorsAreInvisibleFromAnotherMarketplace(t *testing.T) {
	db, one, two := twoMarketplaces(t)
	account := anAccount(t, db, one, "r@example.test")
	if err := db.InTxFor(t.Context(), one, func(tx pgx.Tx) error {
		return insertApp(t.Context(), tx, one, account, "phone", []byte("sealed"), 1, time.Now())
	}); err != nil {
		t.Fatal(err)
	}

	count := func(marketplace string) int {
		var n int
		if err := db.InTxFor(t.Context(), marketplace, func(tx pgx.Tx) error {
			found, err := factorsOf(t.Context(), tx, account)
			n = len(found)
			return err
		}); err != nil {
			t.Fatal(err)
		}
		return n
	}
	if n := count(one); n != 1 {
		t.Fatalf("the marketplace sees %d factors of its own account, want 1", n)
	}
	if n := count(two); n != 0 {
		t.Fatalf("another marketplace sees %d factors of the account, want 0", n)
	}
	// Nor can it write one for an account it cannot see.
	if err := db.InTxFor(t.Context(), two, func(tx pgx.Tx) error {
		return insertApp(t.Context(), tx, one, account, "phone", []byte("sealed"), 1, time.Now())
	}); err == nil {
		t.Fatal("another marketplace wrote a second factor into this one")
	}
}

// An enrolment is the session's, one at a time, and only while it is live.
func TestAnEnrolmentIsTheSessionsWhileItIsLive(t *testing.T) {
	db, one, _ := twoMarketplaces(t)
	account := anAccount(t, db, one, "r@example.test")
	now := time.Now().UTC()
	_, hash, err := NewToken()
	if err != nil {
		t.Fatal(err)
	}
	seedSession(t, db, one, account, hash, now, now, nil)
	var session string
	if err := db.InTxFor(t.Context(), one, func(tx pgx.Tx) error {
		return tx.QueryRow(t.Context(), `SELECT id::text FROM session WHERE token_hash = $1`, hash).Scan(&session)
	}); err != nil {
		t.Fatal(err)
	}

	read := func(at time.Time) (enrolment, error) {
		var e enrolment
		err := db.InTxFor(t.Context(), one, func(tx pgx.Tx) error {
			var err error
			e, err = enrolmentOf(t.Context(), tx, session, MethodApp, at)
			return err
		})
		return e, err
	}
	put := func(secret string) {
		if err := db.InTxFor(t.Context(), one, func(tx pgx.Tx) error {
			return putEnrolment(t.Context(), tx, one, session, account, MethodApp, []byte(secret), nil, now.Add(EnrolmentLifetime))
		}); err != nil {
			t.Fatal(err)
		}
	}

	put("first")
	put("second")
	if e, err := read(now); err != nil || string(e.Secret) != "second" {
		t.Fatalf("the enrolment = %q, %v; want the second, which replaced the first", e.Secret, err)
	}
	if _, err := read(now.Add(EnrolmentLifetime + time.Second)); !errors.Is(err, ErrNoEnrolment) {
		t.Fatalf("an expired enrolment = %v, want ErrNoEnrolment", err)
	}
}

// An id that is not a well-formed UUID is one the account does not have,
// like any other unknown id: PostgreSQL refuses to compare a uuid column
// against text that is not one, so deleteFactor must not hand it one.
func TestDeletingAFactorByAMalformedIDIsNoRows(t *testing.T) {
	db, one, _ := twoMarketplaces(t)
	account := anAccount(t, db, one, "r@example.test")

	for name, id := range map[string]string{"malformed": "not-a-uuid", "empty": ""} {
		t.Run(name, func(t *testing.T) {
			err := db.InTxFor(t.Context(), one, func(tx pgx.Tx) error {
				_, err := deleteFactor(t.Context(), tx, account, id)
				return err
			})
			if !errors.Is(err, pgx.ErrNoRows) {
				t.Fatalf("deleteFactor(%q) = %v, want pgx.ErrNoRows", id, err)
			}
		})
	}
}

// replaceRecoveryCodes stores each code bound to the account, not the plain
// hash recoveryHash gives it, so a stolen database must be searched one
// account at a time (owner's decision, 2026-09-25).
func TestReplaceRecoveryCodesStoresTheBoundHash(t *testing.T) {
	db, one, _ := twoMarketplaces(t)
	account := anAccount(t, db, one, "bound@example.test")
	codes, err := NewRecoveryCodes()
	if err != nil {
		t.Fatal(err)
	}
	plain, ok := recoveryHash(codes[0])
	if !ok {
		t.Fatal("a well-formed code was refused")
	}
	if err := db.InTxFor(t.Context(), one, func(tx pgx.Tx) error {
		hashes := make([][]byte, len(codes))
		for i, code := range codes {
			hash, _ := recoveryHash(code)
			hashes[i] = hash
		}
		return replaceRecoveryCodes(t.Context(), tx, one, account, hashes)
	}); err != nil {
		t.Fatal(err)
	}
	stored := storedRecoveryHashes(t, db, one, account)
	if len(stored) != RecoveryCodeCount {
		t.Fatalf("%d codes stored, want %d", len(stored), RecoveryCodeCount)
	}
	want := boundRecoveryHash(account, plain)
	found := false
	for _, h := range stored {
		if bytes.Equal(h, plain) {
			t.Fatal("the plain, unbound hash was stored")
		}
		if bytes.Equal(h, want) {
			found = true
		}
	}
	if !found {
		t.Fatal("the hash bound to this account was not among the stored codes")
	}
}
