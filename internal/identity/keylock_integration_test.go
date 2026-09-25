//go:build integration

package identity

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

// Two answers from one key to two challenges of the same account must be
// checked one after the other: otherwise both read the stored counter before
// either moves it, and a copy of the key presenting the same counter as the
// real one, or a lower one, passes the clone check (D6) and can even move the
// stored counter backwards. The key's row is locked before the check, so the
// second answer sees the counter the first one left.
func TestAKeysAnswersToTwoChallengesAreCheckedOneAfterTheOther(t *testing.T) {
	for _, tc := range []struct {
		name       string
		first      uint32 // the counter the answer checked first presents
		second     uint32 // the counter the answer checked second presents
		wantClone  string
		wantStored int64
	}{
		{name: "the same counter", first: 1, second: 1,
			wantClone: `{"presented":"1","stored":"1","user_agent":"test"}`, wantStored: 1},
		{name: "a lower counter", first: 2, second: 1,
			wantClone: `{"presented":"1","stored":"2","user_agent":"test"}`, wantStored: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, db, one, _, trail := service(t)
			sealed(t, s)
			session, key, _ := withKey(t, s, db, one, "r@example.test")
			v := visit(one)

			answers := make([]Answer, 2)
			tokens := make([]string, 2)
			for i, count := range []uint32{tc.first, tc.second} {
				tokens[i] = challenged(t, s, one, "r@example.test")
				options, err := s.KeyOptions(t.Context(), v, tokens[i], "")
				if err != nil {
					t.Fatal(err)
				}
				key.count = count
				answers[i] = Answer{Method: MethodKey, Key: key.sign(options)}
			}

			// A answers the first challenge and holds its transaction open,
			// after its check, until told to let go; release is closed once,
			// deferred so that A is always let go, as in
			// TestDeletingFactorsOfOneAccountSerialises.
			holding := make(chan string, 1)
			release := make(chan struct{})
			var releaseOnce sync.Once
			releaseA := func() { releaseOnce.Do(func() { close(release) }) }
			defer releaseA()
			doneA := make(chan error, 1)
			go func() {
				doneA <- s.answerChallenge(t.Context(), v, tokens[0], "", answers[0],
					func(tx pgx.Tx, _ challenge, _ Account, _ time.Time) error {
						var xid string
						if err := tx.QueryRow(t.Context(), `SELECT xid(pg_current_xact_id())::text`).Scan(&xid); err != nil {
							return err
						}
						holding <- xid
						<-release
						return nil
					})
			}()
			var xid string
			select {
			case xid = <-holding:
			case err := <-doneA:
				t.Fatalf("the first answer = %v, want it accepted", err)
			}

			// B answers the second challenge, and is let run until it waits
			// for A: on the key's row, before or after its own check.
			doneB := make(chan error, 1)
			go func() {
				doneB <- s.answerChallenge(t.Context(), v, tokens[1], "", answers[1],
					func(pgx.Tx, challenge, Account, time.Time) error { return nil })
			}()
			waitsFor(t, db, one, xid, doneB)

			releaseA()
			if err := <-doneA; err != nil {
				t.Fatalf("the first answer = %v, want it accepted", err)
			}
			if err := <-doneB; !errors.Is(err, ErrCodeWrong) {
				t.Errorf("the second answer = %v, want ErrCodeWrong", err)
			}
			if stored := storedCount(t, s, db, one, session); stored != tc.wantStored {
				t.Errorf("the stored counter is %d, want %d", stored, tc.wantStored)
			}
			if clone := trail.entry(t, "identity.second_factor_clone_suspected"); string(clone.After) != tc.wantClone {
				t.Fatalf("clone_suspected recorded as %s, want %s", clone.After, tc.wantClone)
			}
		})
	}
}

// waitsFor returns once a transaction waits for the transaction xid to end,
// and fails if done, the waiting transaction's result, arrives first.
func waitsFor(t *testing.T, db serving, marketplace, xid string, done <-chan error) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case err := <-done:
			t.Fatalf("the second answer = %v before the first one committed", err)
		default:
		}
		var waiting int
		if err := db.InTxFor(t.Context(), marketplace, func(tx pgx.Tx) error {
			return tx.QueryRow(t.Context(), `
				SELECT count(*) FROM pg_locks
				 WHERE locktype = 'transactionid' AND transactionid::text = $1 AND NOT granted`, xid).Scan(&waiting)
		}); err != nil {
			t.Fatal(err)
		}
		if waiting > 0 {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("the second answer never waited for the first")
}

// storedCount is the sign counter stored for the account's only key.
func storedCount(t *testing.T, s *Service, db serving, marketplace string, session Session) int64 {
	t.Helper()
	var count int64
	if err := db.InTxFor(t.Context(), marketplace, func(tx pgx.Tx) error {
		_, keys, err := keysOf(t.Context(), tx, session.Account)
		if err != nil {
			return err
		}
		if len(keys) != 1 {
			t.Fatalf("the account has %d keys, want 1", len(keys))
		}
		count = keys[0].SignCount
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return count
}
