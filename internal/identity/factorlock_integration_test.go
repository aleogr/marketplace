//go:build integration

package identity

import (
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

// Two requests that each remove one of an account's two factors must not run
// side by side: under READ COMMITTED, both would count one factor left after
// their own delete and neither would turn 2FA off, even though the account
// ends up with none (final-fix-brief, change 1). The account is locked, so
// the second request's deleteFactor blocks until the first commits.
func TestDeletingFactorsOfOneAccountSerialises(t *testing.T) {
	db, one, _ := twoMarketplaces(t)
	account := anAccount(t, db, one, "r@example.test")
	now := time.Now()
	if err := db.InTxFor(t.Context(), one, func(tx pgx.Tx) error {
		if err := insertApp(t.Context(), tx, one, account, "phone", []byte("sealed-1"), 1, now); err != nil {
			return err
		}
		return insertApp(t.Context(), tx, one, account, "tablet", []byte("sealed-2"), 1, now)
	}); err != nil {
		t.Fatal(err)
	}
	var first, second string
	if err := db.InTxFor(t.Context(), one, func(tx pgx.Tx) error {
		factors, err := factorsOf(t.Context(), tx, account)
		if err != nil {
			return err
		}
		if len(factors) != 2 {
			t.Fatalf("seeded %d factors, want 2", len(factors))
		}
		first, second = factors[0].ID, factors[1].ID
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	// A deletes the first factor and holds its transaction open until told
	// to let go. release is closed exactly once, deferred here so that A is
	// always let go — including when an assertion below fails and unwinds
	// this goroutine through t.Fatal — or its held transaction would keep
	// its connection from ever going back to the pool and the test's own
	// cleanup would hang forever closing it.
	holding := make(chan struct{})
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseA := func() { releaseOnce.Do(func() { close(release) }) }
	defer releaseA()
	doneA := make(chan error, 1)
	go func() {
		doneA <- db.InTxFor(t.Context(), one, func(tx pgx.Tx) error {
			if _, err := deleteFactor(t.Context(), tx, account, first); err != nil {
				return err
			}
			close(holding)
			<-release
			return nil
		})
	}()
	<-holding

	// B deletes the second factor. With the account locked, its deleteFactor
	// cannot return before A commits.
	doneB := make(chan error, 1)
	go func() {
		doneB <- db.InTxFor(t.Context(), one, func(tx pgx.Tx) error {
			_, err := deleteFactor(t.Context(), tx, account, second)
			return err
		})
	}()

	select {
	case <-doneB:
		t.Fatal("transaction B's deleteFactor returned before transaction A committed; the account is not locked")
	case <-time.After(200 * time.Millisecond):
		// B is still blocked, as it must be.
	}

	releaseA()
	if err := <-doneA; err != nil {
		t.Fatalf("transaction A = %v", err)
	}
	if err := <-doneB; err != nil {
		t.Fatalf("transaction B = %v", err)
	}

	if err := db.InTxFor(t.Context(), one, func(tx pgx.Tx) error {
		n, err := countFactors(t.Context(), tx, account)
		if err != nil {
			return err
		}
		if n != 0 {
			t.Fatalf("factors left after both deletions = %d, want 0", n)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
