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

// A recovery code works exactly once, and a new set invalidates the old
// (spec, D6).
func TestARecoveryCodeIsUsableExactlyOnce(t *testing.T) {
	db, one, _ := twoMarketplaces(t)
	account := anAccount(t, db, one, "r@example.test")
	codes, err := NewRecoveryCodes()
	if err != nil {
		t.Fatal(err)
	}
	var hashes [][]byte
	for _, code := range codes {
		hash, _ := recoveryHash(code)
		hashes = append(hashes, hash)
	}
	use := func(code string) bool {
		t.Helper()
		hash, ok := recoveryHash(code)
		if !ok {
			t.Fatalf("%q is not a code", code)
		}
		var used bool
		if err := db.InTxFor(t.Context(), one, func(tx pgx.Tx) error {
			var err error
			used, err = useRecoveryCode(t.Context(), tx, account, hash, time.Now())
			return err
		}); err != nil {
			t.Fatal(err)
		}
		return used
	}
	left := func() (int, int) {
		var issued, remaining int
		if err := db.InTxFor(t.Context(), one, func(tx pgx.Tx) error {
			var err error
			issued, remaining, err = recoveryCodes(t.Context(), tx, account)
			return err
		}); err != nil {
			t.Fatal(err)
		}
		return issued, remaining
	}

	if err := db.InTxFor(t.Context(), one, func(tx pgx.Tx) error {
		return replaceRecoveryCodes(t.Context(), tx, one, account, hashes)
	}); err != nil {
		t.Fatal(err)
	}
	if !use(codes[0]) {
		t.Fatal("a fresh code was refused")
	}
	if use(codes[0]) {
		t.Fatal("a code was accepted twice")
	}
	if issued, remaining := left(); issued != 10 || remaining != 9 {
		t.Fatalf("issued %d, left %d; want 10 and 9", issued, remaining)
	}

	fresh, err := NewRecoveryCodes()
	if err != nil {
		t.Fatal(err)
	}
	var freshHashes [][]byte
	for _, code := range fresh {
		hash, _ := recoveryHash(code)
		freshHashes = append(freshHashes, hash)
	}
	if err := db.InTxFor(t.Context(), one, func(tx pgx.Tx) error {
		return replaceRecoveryCodes(t.Context(), tx, one, account, freshHashes)
	}); err != nil {
		t.Fatal(err)
	}
	if use(codes[1]) {
		t.Fatal("a code of the old set still works after a new set was issued")
	}
	if !use(fresh[1]) {
		t.Fatal("a code of the new set was refused")
	}
}
