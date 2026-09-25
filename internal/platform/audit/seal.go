package audit

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/aleogr/marketplace/internal/platform/keys"
)

// SealFor encrypts plaintext with person's own key, creating the key the first
// time, in the caller's transaction.
//
// It is for what another part of the platform must keep about a person and
// must stop being readable when that person asks to be erased: a second
// factor's secret, for one (F14 spec, D5). Destroying the key then erases it
// together with everything the log says about them, and there is nothing else
// to find. The key is the one the log uses; how the log uses it does not
// change.
func (k *Keys) SealFor(ctx context.Context, tx pgx.Tx, person string, plaintext []byte) ([]byte, error) {
	if err := personal(person); err != nil {
		return nil, err
	}
	key, err := k.open(ctx, tx, person)
	if err != nil {
		return nil, err
	}
	sealed, err := keys.Seal(key, plaintext)
	if err != nil {
		return nil, fmt.Errorf("cannot seal for %s: %w", person, err)
	}
	return sealed, nil
}

// OpenFor decrypts what SealFor sealed for person. Once the person's key is
// destroyed the error wraps keys.ErrDestroyed: what was sealed is gone, which
// is what was asked for.
func (k *Keys) OpenFor(ctx context.Context, tx pgx.Tx, person string, sealed []byte) ([]byte, error) {
	if err := personal(person); err != nil {
		return nil, err
	}
	key, err := k.open(ctx, tx, person)
	if err != nil {
		return nil, err
	}
	plaintext, err := keys.Open(key, sealed)
	if err != nil {
		return nil, fmt.Errorf("cannot open what was sealed for %s: %w", person, err)
	}
	return plaintext, nil
}

// personal refuses a key that belongs to nobody: the platform's own is never
// destroyed, so what it sealed could never be erased.
func personal(person string) error {
	if person == "" || person == Platform {
		return errors.New("only a person's own key seals what is about them")
	}
	return nil
}
