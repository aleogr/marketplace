package identity

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Params are argon2id's costs. Memory is in KiB.
type Params struct {
	Memory  uint32
	Time    uint32
	Threads uint8
	KeyLen  uint32
	SaltLen uint32
}

// Floor is OWASP's minimum for argon2id (19 MiB, two passes, one lane). It is
// what the service hashes with until the benchmark on its own CPU chooses
// stronger parameters (spec, D5), and what the benchmark falls back to.
var Floor = Params{Memory: 19456, Time: 2, Threads: 1, KeyLen: 32, SaltLen: 16}

// Current is what new hashes are made with: what `bench-password` chose on the
// lab's Cloud Run CPU (1 vCPU, 512 MiB) on 2026-09-23, 152 ms per hash
// against a 250 ms budget (cmd/marketplace/bench.go, docs/infrastructure.md).
// A hash made with other parameters is remade at its owner's next sign-in.
var Current = Params{Memory: 65536, Time: 3, Threads: 1, KeyLen: 32, SaltLen: 16}

// maxEncodedLen bounds the salt and the key read back from a stored hash. Ours
// are 16 and 32 bytes; anything past this is not a hash this service wrote.
const maxEncodedLen = 1024

// maxMemoryKiB and maxTime bound the cost parameters read back from a stored
// hash, which is untrusted input. argon2.IDKey panics when Time or Threads is
// zero, and an unbounded Memory allocates without limit — a hash we did not
// write ourselves must not be able to crash the process or pin a slot for an
// unbounded time.
const (
	maxMemoryKiB = 262144 // 256 MiB
	maxTime      = 10
)

var errNotPHC = errors.New("identity: not an argon2id PHC string")

// Hasher hashes and verifies passwords, a bounded number at a time.
//
// The bound is the instance's memory: 80 concurrent requests on 512 MiB cannot
// each hold tens of megabytes (spec, D5). A request beyond it waits, and gives
// up with its context. No caller holds a database transaction while it waits
// or hashes: a connection of a four-connection pool held for the length of a
// hash is a connection every other request waits for (internal/platform/db).
type Hasher struct {
	params Params
	slots  chan struct{}
	dummy  string
}

// NewHasher returns a hasher with params, allowing concurrent hashes at once.
func NewHasher(params Params, concurrent int) *Hasher {
	h := &Hasher{params: params, slots: make(chan struct{}, concurrent)}
	h.dummy = h.encode([]byte("identity: the dummy that equalises timing"))
	return h
}

func (h *Hasher) acquire(ctx context.Context) error {
	select {
	case h.slots <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (h *Hasher) release() { <-h.slots }

// Hash returns the PHC string for password.
func (h *Hasher) Hash(ctx context.Context, password string) (string, error) {
	if err := h.acquire(ctx); err != nil {
		return "", err
	}
	defer h.release()
	return h.encode([]byte(normalise(password))), nil
}

func (h *Hasher) encode(password []byte) string {
	salt := make([]byte, h.params.SaltLen)
	_, _ = rand.Read(salt)
	key := argon2.IDKey(password, salt, h.params.Time, h.params.Memory, h.params.Threads, h.params.KeyLen)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", argon2.Version,
		h.params.Memory, h.params.Time, h.params.Threads,
		base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key))
}

// Verify reports whether password matches encoded, and whether encoded was made
// with parameters other than this hasher's, so the caller can rehash it.
//
// The derived key is compared with crypto/subtle.ConstantTimeCompare, so how
// long the comparison takes says nothing about how much of it matched
// (spec, D6).
func (h *Hasher) Verify(ctx context.Context, password, encoded string) (ok, stale bool, err error) {
	params, salt, key, err := decode(encoded)
	if err != nil {
		return false, false, err
	}
	if err := h.acquire(ctx); err != nil {
		return false, false, err
	}
	defer h.release()
	got := argon2.IDKey([]byte(normalise(password)), salt, params.Time, params.Memory, params.Threads, params.KeyLen)
	return subtle.ConstantTimeCompare(got, key) == 1, params != h.params, nil
}

// Waste spends what a verification costs, so an unknown address takes as long
// as a wrong password (spec, D7).
func (h *Hasher) Waste(ctx context.Context, password string) {
	_, _, _ = h.Verify(ctx, password, h.dummy)
}

// decode reads a PHC string back into its parameters, salt and key.
func decode(encoded string) (Params, []byte, []byte, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != fmt.Sprintf("v=%d", argon2.Version) {
		return Params{}, nil, nil, errNotPHC
	}
	var p Params
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.Memory, &p.Time, &p.Threads); err != nil {
		return Params{}, nil, nil, errNotPHC
	}
	// Bounded before argon2 ever sees them: t=0 or p=0 panics inside
	// argon2.IDKey, and an unbounded m allocates without limit.
	if p.Memory < 1 || p.Memory > maxMemoryKiB || p.Time < 1 || p.Time > maxTime || p.Threads < 1 {
		return Params{}, nil, nil, errNotPHC
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return Params{}, nil, nil, errNotPHC
	}
	key, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return Params{}, nil, nil, errNotPHC
	}
	// Bounded before they are converted, so the lengths fit the uint32 that
	// argon2 takes.
	saltLen, keyLen := len(salt), len(key)
	if saltLen < 1 || saltLen > maxEncodedLen || keyLen < 1 || keyLen > maxEncodedLen {
		return Params{}, nil, nil, errNotPHC
	}
	p.SaltLen, p.KeyLen = uint32(saltLen), uint32(keyLen)
	return p, salt, key, nil
}
