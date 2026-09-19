package keys_test

import (
	"context"
	"errors"
	"testing"

	"github.com/aleogr/marketplace/internal/platform/keys"
	"github.com/aleogr/marketplace/internal/platform/keys/local"
)

func TestSealedContentComesBack(t *testing.T) {
	key, err := keys.New()
	if err != nil {
		t.Fatalf("keys.New() = %v", err)
	}

	sealed, err := keys.Seal(key, []byte("the origin address and what changed"))
	if err != nil {
		t.Fatalf("Seal() = %v", err)
	}

	opened, err := keys.Open(key, sealed)
	if err != nil {
		t.Fatalf("Open() = %v", err)
	}
	if string(opened) != "the origin address and what changed" {
		t.Errorf("opened %q", opened)
	}
}

// Two messages sealed with one key must not be recognisable as the same
// content: a log full of identical ciphertexts tells a reader who never gets a
// key which records are about the same thing.
func TestTheSameContentSealsDifferentlyEachTime(t *testing.T) {
	key, err := keys.New()
	if err != nil {
		t.Fatalf("keys.New() = %v", err)
	}

	first, err := keys.Seal(key, []byte("192.0.2.10"))
	if err != nil {
		t.Fatalf("Seal() = %v", err)
	}
	second, err := keys.Seal(key, []byte("192.0.2.10"))
	if err != nil {
		t.Fatalf("Seal() = %v", err)
	}

	if string(first) == string(second) {
		t.Error("the same content sealed to the same bytes twice")
	}
}

// The property the audit chain leans on: a ciphertext that was altered does not
// open, so tampering is noticed rather than decrypted into nonsense.
func TestAnAlteredSealDoesNotOpen(t *testing.T) {
	key, err := keys.New()
	if err != nil {
		t.Fatalf("keys.New() = %v", err)
	}

	sealed, err := keys.Seal(key, []byte("what changed"))
	if err != nil {
		t.Fatalf("Seal() = %v", err)
	}
	sealed[len(sealed)-1]++

	if _, err := keys.Open(key, sealed); err == nil {
		t.Error("an altered seal opened")
	}
}

func TestAnotherKeyDoesNotOpenIt(t *testing.T) {
	mine, err := keys.New()
	if err != nil {
		t.Fatalf("keys.New() = %v", err)
	}
	theirs, err := keys.New()
	if err != nil {
		t.Fatalf("keys.New() = %v", err)
	}

	sealed, err := keys.Seal(mine, []byte("what changed"))
	if err != nil {
		t.Fatalf("Seal() = %v", err)
	}

	if _, err := keys.Open(theirs, sealed); err == nil {
		t.Error("another person's key opened this person's content")
	}
}

func TestAKeyOfTheWrongSizeIsRefused(t *testing.T) {
	if _, err := keys.Seal([]byte("too short"), []byte("x")); err == nil {
		t.Error("Seal() accepted a key of the wrong size")
	}
	if _, err := local.New([]byte("too short")); err == nil {
		t.Error("local.New() accepted a key of the wrong size")
	}
}

func TestTheLocalKeeperWrapsAndUnwraps(t *testing.T) {
	keeper, err := local.Generate()
	if err != nil {
		t.Fatalf("local.Generate() = %v", err)
	}

	if err := keys.Check(t.Context(), keeper); err != nil {
		t.Errorf("Check() = %v", err)
	}
}

// A key wrapped by one keeper is not a key another can open, which is why every
// wrapped key is stored beside the name of the keeper that wrapped it.
func TestAKeyWrappedByAnotherKeeperDoesNotUnwrap(t *testing.T) {
	mine, err := local.Generate()
	if err != nil {
		t.Fatalf("local.Generate() = %v", err)
	}
	theirs, err := local.Generate()
	if err != nil {
		t.Fatalf("local.Generate() = %v", err)
	}

	key, err := keys.New()
	if err != nil {
		t.Fatalf("keys.New() = %v", err)
	}
	wrapped, err := mine.Wrap(t.Context(), key)
	if err != nil {
		t.Fatalf("Wrap() = %v", err)
	}

	if _, err := theirs.Unwrap(t.Context(), wrapped); err == nil {
		t.Error("a key wrapped by one keeper was unwrapped by another")
	}
}

// broken is a keeper that wraps and cannot unwrap, which is what a deployment
// looks like when its identity may encrypt and not decrypt.
type broken struct{ keys.Keeper }

func (b broken) Unwrap(context.Context, []byte) ([]byte, error) {
	return nil, errors.New("permission denied")
}

func TestCheckCatchesAKeeperThatOnlyWraps(t *testing.T) {
	working, err := local.Generate()
	if err != nil {
		t.Fatalf("local.Generate() = %v", err)
	}

	if err := keys.Check(t.Context(), broken{Keeper: working}); err == nil {
		t.Error("Check() passed a keeper that cannot unwrap what it wrapped")
	}
}

func TestParseReadsABase64Key(t *testing.T) {
	for name, encoded := range map[string]string{
		"not base64": "not a key",
		"too short":  "c2hvcnQ=",
		"empty":      "",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := local.Parse(encoded); err == nil {
				t.Error("local.Parse() accepted a key it cannot use")
			}
		})
	}
}
