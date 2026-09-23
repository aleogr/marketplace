package identity

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

// cheap keeps the unit tests fast; the real parameters are measured by the
// benchmark (cmd/marketplace, `bench-password`).
var cheap = Params{Memory: 64, Time: 1, Threads: 1, KeyLen: 32, SaltLen: 16}

func TestHashVerifiesAndRejects(t *testing.T) {
	h := NewHasher(cheap, 2)
	encoded, err := h.Hash(context.Background(), "correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(encoded, "$argon2id$v=19$m=64,t=1,p=1$") {
		t.Fatalf("not a PHC argon2id string: %q", encoded)
	}
	if ok, stale, err := h.Verify(context.Background(), "correct horse battery", encoded); !ok || stale || err != nil {
		t.Fatalf("Verify(right) = %v, %v, %v; want true, false, nil", ok, stale, err)
	}
	if ok, _, err := h.Verify(context.Background(), "wrong horse battery", encoded); ok || err != nil {
		t.Fatalf("Verify(wrong) = %v, %v; want false, nil", ok, err)
	}
}

func TestTwoHashesOfOnePasswordDiffer(t *testing.T) {
	h := NewHasher(cheap, 1)
	a, _ := h.Hash(context.Background(), "same password here")
	b, _ := h.Hash(context.Background(), "same password here")
	if a == b {
		t.Fatal("two hashes of one password are equal; the salt is not random")
	}
}

func TestAHashWithOtherParametersIsStale(t *testing.T) {
	old := NewHasher(cheap, 1)
	encoded, _ := old.Hash(context.Background(), "passphrase long enough")
	current := NewHasher(Params{Memory: 128, Time: 1, Threads: 1, KeyLen: 32, SaltLen: 16}, 1)
	ok, stale, err := current.Verify(context.Background(), "passphrase long enough", encoded)
	if !ok || !stale || err != nil {
		t.Fatalf("Verify = %v, %v, %v; want true, true, nil", ok, stale, err)
	}
}

// Every parameter a hash records is one a change of Current must reach, not
// only the memory.
func TestAHashWithAnyOtherParameterIsStale(t *testing.T) {
	for name, other := range map[string]Params{
		"time":    {Memory: 64, Time: 2, Threads: 1, KeyLen: 32, SaltLen: 16},
		"threads": {Memory: 64, Time: 1, Threads: 2, KeyLen: 32, SaltLen: 16},
		"key":     {Memory: 64, Time: 1, Threads: 1, KeyLen: 16, SaltLen: 16},
	} {
		encoded, err := NewHasher(other, 1).Hash(context.Background(), "passphrase long enough")
		if err != nil {
			t.Fatal(err)
		}
		ok, stale, err := NewHasher(cheap, 1).Verify(context.Background(), "passphrase long enough", encoded)
		if !ok || !stale || err != nil {
			t.Errorf("%s differs: Verify = %v, %v, %v; want true, true, nil", name, ok, stale, err)
		}
	}
}

func TestNormalisedFormsVerifyAlike(t *testing.T) {
	h := NewHasher(cheap, 1)
	composed := "senha com cedilha ç ok"    // U+00E7, one code point
	decomposed := "senha com cedilha ç ok" // 'c' followed by U+0327, a combining cedilla
	if composed == decomposed {
		// Guards against an editor or a copy/paste silently re-normalising
		// the decomposed literal back to its composed form, which would
		// make this test pass even with no normalisation in Verify at all.
		t.Fatal("composed and decomposed literals are byte-identical; the test is vacuous")
	}
	encoded, _ := h.Hash(context.Background(), composed)
	if ok, _, _ := h.Verify(context.Background(), decomposed, encoded); !ok {
		t.Fatal("the same password typed on another keyboard did not verify")
	}
}

func TestAMalformedHashIsAnErrorNotAMatch(t *testing.T) {
	h := NewHasher(cheap, 1)
	for _, encoded := range []string{
		"",
		"$argon2i$v=19$m=64,t=1,p=1$c2FsdHNhbHRzYWx0c2FsdA$a2V5",
		"$argon2id$v=19$m=64,t=1,p=1$c2FsdHNhbHRzYWx0c2FsdA$",                                  // no key
		"$argon2id$v=19$m=64,t=1,p=1$" + strings.Repeat("A", 2000) + "$a2V5",                   // salt beyond the bound
		"$argon2id$v=19$m=64,t=0,p=1$c2FsdHNhbHRzYWx0c2FsdA$a2V5",                              // t=0 would panic inside argon2
		"$argon2id$v=19$m=64,t=1,p=0$c2FsdHNhbHRzYWx0c2FsdA$a2V5",                              // p=0 would panic inside argon2
		fmt.Sprintf("$argon2id$v=19$m=%d,t=1,p=1$c2FsdHNhbHRzYWx0c2FsdA$a2V5", maxMemoryKiB+1), // m beyond the ceiling
		fmt.Sprintf("$argon2id$v=19$m=64,t=%d,p=1$c2FsdHNhbHRzYWx0c2FsdA$a2V5", maxTime+1),     // t beyond the ceiling
	} {
		if ok, _, err := h.Verify(context.Background(), "whatever", encoded); ok || err == nil {
			t.Errorf("Verify(%.40q) = %v, %v; want false and an error", encoded, ok, err)
		}
	}
}

func TestHashingWaitsForASlotAndHonoursTheContext(t *testing.T) {
	h := NewHasher(cheap, 1)
	h.slots <- struct{}{} // occupy the only slot
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := h.Hash(ctx, "any password at all"); err == nil {
		t.Fatal("Hash did not give up when its context was cancelled while waiting")
	}
}

// The deployed parameters may only ever be stronger than OWASP's floor
// (spec, D5), whatever the benchmark measured.
func TestCurrentIsNoWeakerThanTheFloor(t *testing.T) {
	if uint64(Current.Memory)*uint64(Current.Time) < uint64(Floor.Memory)*uint64(Floor.Time) ||
		Current.Threads < Floor.Threads || Current.KeyLen < Floor.KeyLen || Current.SaltLen < Floor.SaltLen {
		t.Fatalf("Current = %+v is weaker than Floor = %+v", Current, Floor)
	}
}

func TestVerifyingWaitsForASlotAndHonoursTheContext(t *testing.T) {
	h := NewHasher(cheap, 1)
	encoded, err := h.Hash(context.Background(), "any password at all")
	if err != nil {
		t.Fatal(err)
	}
	h.slots <- struct{}{} // occupy the only slot
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if ok, _, err := h.Verify(ctx, "any password at all", encoded); ok || !errors.Is(err, context.Canceled) {
		t.Fatalf("Verify with a cancelled context = %v, %v; want false, context.Canceled", ok, err)
	}
}

func TestWasteSpendsASlotAndGivesItBack(t *testing.T) {
	h := NewHasher(cheap, 1)
	h.Waste(context.Background(), "any password at all")
	if len(h.slots) != 0 {
		t.Fatal("Waste kept its slot")
	}

	// And it gives up with its context, like a verification.
	h.slots <- struct{}{} // occupy the only slot
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	done := make(chan struct{})
	go func() {
		h.Waste(ctx, "any password at all")
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Waste did not give up when its context was cancelled while waiting")
	}
}
