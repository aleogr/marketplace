// Package keys is how the platform protects what it must be able to destroy.
//
// Every person whose data appears in the audit log has a key of their own. The
// log holds only what that key can open, and a deletion request is fulfilled by
// destroying the key: the record and the hash chain stay exactly as they were,
// and the content becomes unrecoverable (docs/requirements.md, sections 21 and
// 18.3). Erasing the records themselves is not an option — an audit log with a
// hole in it is not an audit log.
//
// Two levels, which is what makes both halves possible. A data key encrypts one
// person's content and lives in the database, wrapped; a Keeper wraps and
// unwraps it and never leaves the place it is kept. Destroying the wrapped key
// destroys that person's content and nothing else.
package keys

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/subtle"
	"errors"
	"fmt"
)

// KeySize is the length of a data key, in bytes: AES-256.
const KeySize = 32

// Keeper wraps and unwraps data keys.
//
// It is a port (docs/requirements.md, section 25): the deployment uses Cloud
// KMS, where the wrapping key cannot be read by anything, including this
// process; tests use an adapter that does the same arithmetic with a key in
// memory. What neither of them does is hand out the wrapping key.
type Keeper interface {
	// Name is which keeper wrapped a key, recorded beside it: a key wrapped by
	// one cannot be unwrapped by another, and knowing which is which is the
	// difference between a migration and a loss.
	Name() string
	Wrap(ctx context.Context, key []byte) ([]byte, error)
	Unwrap(ctx context.Context, wrapped []byte) ([]byte, error)
}

// ErrDestroyed is what unwrapping a key that no longer exists returns.
//
// It is not a failure. It is the system working: somebody asked for their data
// to be erased, and this is what erased looks like from the inside.
var ErrDestroyed = errors.New("the key was destroyed")

// New returns a data key nobody has seen.
func New() ([]byte, error) {
	key := make([]byte, KeySize)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("cannot generate a key: %w", err)
	}
	return key, nil
}

// Seal encrypts plaintext with key, and returns the nonce and the ciphertext
// as one value.
//
// AES-GCM: the ciphertext cannot be altered without the alteration being
// noticed on opening, which matters here because the ciphertext is part of what
// the audit chain hashes.
func Seal(key, plaintext []byte) ([]byte, error) {
	sealer, err := gcm(key)
	if err != nil {
		return nil, err
	}

	// A nonce per message, never reused with the same key: reuse is what
	// breaks GCM, and random is how a process with no shared counter gets
	// there.
	nonce := make([]byte, sealer.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("cannot generate a nonce: %w", err)
	}
	return sealer.Seal(nonce, nonce, plaintext, nil), nil
}

// Open decrypts what Seal produced.
func Open(key, sealed []byte) ([]byte, error) {
	opener, err := gcm(key)
	if err != nil {
		return nil, err
	}

	size := opener.NonceSize()
	if len(sealed) < size {
		return nil, errors.New("the sealed value is too short to hold a nonce")
	}

	plaintext, err := opener.Open(nil, sealed[:size], sealed[size:], nil)
	if err != nil {
		// Either the wrong key or an altered ciphertext, and the difference is
		// not knowable from here — which is the property that makes it worth
		// having.
		return nil, fmt.Errorf("cannot open the sealed value: %w", err)
	}
	return plaintext, nil
}

func gcm(key []byte) (cipher.AEAD, error) {
	if len(key) != KeySize {
		return nil, fmt.Errorf("a key is %d bytes, not %d", KeySize, len(key))
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("cannot use the key: %w", err)
	}

	sealer, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("cannot prepare the cipher: %w", err)
	}
	return sealer, nil
}

// Check proves a keeper can do both halves of its job.
//
// It wraps a value nobody needs and unwraps it again. A deployment whose
// service account may encrypt but not decrypt is configured, looks configured,
// and loses everything it writes; this is what tells the difference, at
// start-up, for the cost of one call.
func Check(ctx context.Context, keeper Keeper) error {
	probe, err := New()
	if err != nil {
		return err
	}

	wrapped, err := keeper.Wrap(ctx, probe)
	if err != nil {
		return fmt.Errorf("%s cannot wrap a key: %w", keeper.Name(), err)
	}

	unwrapped, err := keeper.Unwrap(ctx, wrapped)
	if err != nil {
		return fmt.Errorf("%s cannot unwrap what it wrapped: %w", keeper.Name(), err)
	}

	if !equal(probe, unwrapped) {
		return fmt.Errorf("%s returned a different key than it was given", keeper.Name())
	}
	return nil
}

// equal compares in constant time, out of habit rather than need: the value is
// this function's own.
func equal(a, b []byte) bool { return subtle.ConstantTimeCompare(a, b) == 1 }
