package identity

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/aleogr/marketplace/internal/identity/breached"
)

// quiet discards what the flows log.
var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

// untouched is a Transactor that fails the test if a flow opens a transaction.
type untouched struct{ t *testing.T }

func (u untouched) InTxFor(context.Context, string, func(pgx.Tx) error) error {
	u.t.Error("the flow opened a transaction")
	return errors.New("no transaction expected")
}

// A password no person could have set is refused before anything is spent on
// it: no normalisation, no argon2, no transaction, and the same answer as a
// wrong password (D7).
func TestAnOversizedPasswordIsRefusedForNothing(t *testing.T) {
	h := NewHasher(cheap, 1)
	h.slots <- struct{}{} // occupy the only slot: a hash would wait for it
	s := NewService(untouched{t}, h, breached.Fake{}, nil, quiet)

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	huge := strings.Repeat("a", maxPasswordBytes+1)
	if _, err := s.SignIn(ctx, Visit{Marketplace: "m"}, "r@example.test", huge); !errors.Is(err, ErrCredentials) {
		t.Fatalf("SignIn(oversized password) = %v, want ErrCredentials", err)
	}
}

// A password change applies the same cap to both passwords it is given: an
// oversized current one is refused as a wrong one, and an oversized new one as
// too long, before anything is spent on either.
func TestAnOversizedPasswordChangeIsRefusedForNothing(t *testing.T) {
	h := NewHasher(cheap, 1)
	h.slots <- struct{}{} // occupy the only slot: a hash would wait for it
	s := NewService(untouched{t}, h, breached.Fake{}, nil, quiet)

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	huge := strings.Repeat("a", maxPasswordBytes+1)
	session := Session{ID: "s", Account: Account{ID: "a"}}
	if _, err := s.ChangePassword(ctx, Visit{Marketplace: "m"}, session, huge, "a brand new passphrase"); !errors.Is(err, ErrCredentials) {
		t.Errorf("ChangePassword(oversized current) = %v, want ErrCredentials", err)
	}
	if _, err := s.ChangePassword(ctx, Visit{Marketplace: "m"}, session, "correct horse battery staple", huge); !errors.Is(err, ErrPasswordLong) {
		t.Errorf("ChangePassword(oversized new) = %v, want ErrPasswordLong", err)
	}
}
