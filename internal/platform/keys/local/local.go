// Package local is the Keeper an environment without Cloud KMS uses.
//
// It performs the same operation with a key this process holds, which is the
// whole difference: a keeper that cannot be asked for its key is what makes
// destroying a data key final, and this one can be read by anything that can
// read the process's environment. It exists so that tests and a local run
// exercise the same code path as a deployment, and it says so in the log
// wherever it is used (docs/requirements.md, section 25 — tests use fakes,
// never real services).
package local

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/aleogr/marketplace/internal/platform/keys"
)

// Name is what a key wrapped by this keeper is recorded as. A deployment that
// finds it in its own database is looking at a key that was never protected.
const Name = "local"

// Keeper wraps data keys with one key of its own.
type Keeper struct{ master []byte }

// New returns a keeper wrapping with master, which must be a 32-byte key.
func New(master []byte) (*Keeper, error) {
	if len(master) != keys.KeySize {
		return nil, fmt.Errorf("the local keeper needs a %d-byte key, not %d",
			keys.KeySize, len(master))
	}
	return &Keeper{master: master}, nil
}

// Parse reads a base64 key, as configuration carries one.
func Parse(encoded string) (*Keeper, error) {
	master, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("the local key is not base64: %w", err)
	}
	return New(master)
}

// Generate returns a keeper with a key nobody kept.
//
// What it wraps cannot be unwrapped by the next process, which is exactly
// right for a test and exactly wrong for anything else.
func Generate() (*Keeper, error) {
	master, err := keys.New()
	if err != nil {
		return nil, err
	}
	return New(master)
}

// Name reports which keeper this is.
func (k *Keeper) Name() string { return Name }

// Wrap encrypts a data key.
func (k *Keeper) Wrap(_ context.Context, key []byte) ([]byte, error) {
	return keys.Seal(k.master, key)
}

// Unwrap decrypts one.
func (k *Keeper) Unwrap(_ context.Context, wrapped []byte) ([]byte, error) {
	return keys.Open(k.master, wrapped)
}
