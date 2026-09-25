//go:build integration

package audit_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/aleogr/marketplace/internal/platform/audit"
	"github.com/aleogr/marketplace/internal/platform/keys"
	"github.com/aleogr/marketplace/internal/platform/keys/local"
)

// What another package keeps about a person, sealed with that person's key,
// opens until the key is destroyed and never after; and the log, which shares
// the key, is erased by the same destruction and by nothing else (F14 spec,
// D5).
func TestWhatIsSealedForAPersonOpensUntilTheirKeyIsDestroyed(t *testing.T) {
	site := assemble(t)
	keeper, err := local.Generate()
	if err != nil {
		t.Fatal(err)
	}
	personal := audit.NewKeys(keeper)
	trail := audit.NewLog(personal, quiet())
	marketplace := site.marketplace(t, "audit-seal")
	buyer := person(t)

	var sealed []byte
	if err := site.pool.InTx(t.Context(), func(tx pgx.Tx) error {
		if sealed, err = personal.SealFor(t.Context(), tx, buyer, []byte("a secret of the buyer's")); err != nil {
			return err
		}
		return trail.Append(t.Context(), tx, audit.Entry{
			Marketplace: marketplace,
			Actor:       audit.Actor{ID: buyer, Kind: audit.Buyer},
			Action:      "identity.second_factor_added",
			Subject:     audit.Subject{Kind: "account", ID: buyer, Person: buyer},
			After:       json.RawMessage(`{"kind":"totp"}`),
		})
	}); err != nil {
		t.Fatalf("sealing and appending = %v", err)
	}
	if string(sealed) == "a secret of the buyer's" {
		t.Fatal("SealFor returned the plaintext")
	}

	opened := func() ([]byte, error) {
		var plaintext []byte
		err := site.pool.InTx(t.Context(), func(tx pgx.Tx) error {
			var err error
			plaintext, err = personal.OpenFor(t.Context(), tx, buyer, sealed)
			return err
		})
		return plaintext, err
	}
	if plaintext, err := opened(); err != nil || string(plaintext) != "a secret of the buyer's" {
		t.Fatalf("OpenFor = %q, %v", plaintext, err)
	}

	// Another person's key does not open it.
	if err := site.pool.InTx(t.Context(), func(tx pgx.Tx) error {
		_, err := personal.OpenFor(t.Context(), tx, person(t), sealed)
		return err
	}); err == nil {
		t.Fatal("another person's key opened what was sealed for the buyer")
	}

	if err := site.pool.InTx(t.Context(), func(tx pgx.Tx) error {
		return personal.Destroy(t.Context(), tx, buyer)
	}); err != nil {
		t.Fatalf("Destroy() = %v", err)
	}
	if _, err := opened(); !errors.Is(err, keys.ErrDestroyed) {
		t.Fatalf("OpenFor after the key was destroyed = %v, want keys.ErrDestroyed", err)
	}

	var record audit.Opened
	if err := site.pool.InTx(t.Context(), func(tx pgx.Tx) error {
		var err error
		record, err = trail.Open(t.Context(), tx, marketplace, 1)
		return err
	}); err != nil {
		t.Fatalf("Open() = %v", err)
	}
	if !record.StateErased || record.Action != "identity.second_factor_added" {
		t.Fatalf("the log's record after the destruction = %+v, want its state erased and the record kept", record)
	}
}

// The platform's own key is nobody's: nothing about a person is sealed with it.
func TestNothingIsSealedForThePlatformOrForNobody(t *testing.T) {
	site := assemble(t)
	keeper, err := local.Generate()
	if err != nil {
		t.Fatal(err)
	}
	personal := audit.NewKeys(keeper)
	for _, who := range []string{audit.Platform, ""} {
		if err := site.pool.InTx(t.Context(), func(tx pgx.Tx) error {
			_, err := personal.SealFor(t.Context(), tx, who, []byte("x"))
			return err
		}); err == nil {
			t.Errorf("SealFor(%q) sealed something", who)
		}
	}
}
