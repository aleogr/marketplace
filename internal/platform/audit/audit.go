// Package audit records what was done, by whom, from where, and what changed.
//
// Three promises hold it together (docs/requirements.md, section 21). A record
// is never changed or removed — the database refuses both. A record carries the
// hash of the one before it, so altering one out of band breaks every hash
// after it. And what a record says about a person is encrypted with that
// person's own key, so a deletion request destroys the key and leaves the log
// whole (section 18.3): a log with a hole in it would prove nothing about the
// records around the hole.
package audit

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"time"
)

// Kind is what sort of actor performed an operation.
type Kind string

const (
	// Staff is the owner or a platform operator.
	Staff Kind = "staff"
	// Store is a seller acting in their own store.
	Store Kind = "store"
	// Buyer is a person buying.
	Buyer Kind = "buyer"
	// System is the platform acting on its own: a scheduled job, a provider's
	// webhook, a migration.
	System Kind = "system"
)

// Platform is the identifier of the platform's own key.
//
// Everything a record holds about people is encrypted, and so is everything
// else it holds that could grow into personal data — which leaves no clear-text
// path to audit by mistake. Work nobody personally did is encrypted with this
// key, which is never destroyed because there is nobody to ask for its
// destruction.
const Platform = "00000000-0000-0000-0000-000000000000"

// Actor is who did it, by internal identifier only.
type Actor struct {
	// ID is the internal identifier of the person. Empty for the system.
	ID   string
	Kind Kind
}

// Subject is what was acted on.
type Subject struct {
	// Kind is what sort of thing it is: `marketplace`, `parameter`, `order`.
	Kind string
	// ID identifies it. Text rather than an identifier, because a parameter is
	// identified by its name.
	ID string
	// Person is the internal identifier of the person the state is about, when
	// there is one. It decides whose key protects the state before and after:
	// staff editing a buyer's record leaves the buyer's data behind the
	// buyer's key, which is what makes the buyer's deletion request mean
	// something.
	Person string
}

// Entry is one operation, as the caller describes it.
type Entry struct {
	// Marketplace is whose operation this is, empty for the platform's own. It
	// also selects the chain the record is appended to.
	Marketplace string
	Actor       Actor
	// Action names what was done, in the past tense and in the platform's own
	// vocabulary: `marketplace.created`, `parameter.changed`.
	Action  string
	Subject Subject
	// From is the address the operation came from.
	From string
	// Before and After are the state, as JSON. Either may be absent: a
	// creation has no before, a deletion no after.
	Before json.RawMessage
	After  json.RawMessage
}

// origin is what is sealed with the actor's key.
type origin struct {
	From string `json:"from,omitempty"`
}

// state is what is sealed with the subject's key.
type state struct {
	Before json.RawMessage `json:"before,omitempty"`
	After  json.RawMessage `json:"after,omitempty"`
}

// record is one row, as it is written and as it is read back for verification.
type record struct {
	Marketplace   string
	Sequence      int64
	RecordedAt    time.Time
	ActorID       string
	ActorKind     string
	Action        string
	SubjectKind   string
	SubjectID     string
	ActorSecret   []byte
	SubjectSecret []byte
	PreviousHash  []byte
	Hash          []byte
}

// digest is the hash of a record, and the one place that decides what the
// chain covers.
//
// Every field is written with its length in front of it, so that no two
// different records can produce the same bytes by moving a boundary — the
// classic way a chain is defeated without altering a single byte of content.
// The ciphertexts are covered too: changing what a record says about somebody,
// even unreadably, breaks the chain.
func digest(previous []byte, r record) []byte {
	sum := sha256.New()
	write := func(value []byte) {
		var length [8]byte
		binary.BigEndian.PutUint64(length[:], uint64(len(value)))
		sum.Write(length[:])
		sum.Write(value)
	}

	// A number is written as itself, signed, rather than converted: writing it
	// through binary.Write keeps the value and its type together, and a hash
	// never fails to be written to.
	number := func(value int64) { _ = binary.Write(sum, binary.BigEndian, value) }

	write(previous)
	write([]byte(r.Marketplace))
	number(r.Sequence)

	// Microseconds, because that is the resolution PostgreSQL keeps: hashing
	// nanoseconds would produce a digest the stored row can never reproduce.
	number(r.RecordedAt.UnixMicro())

	write([]byte(r.ActorID))
	write([]byte(r.ActorKind))
	write([]byte(r.Action))
	write([]byte(r.SubjectKind))
	write([]byte(r.SubjectID))
	write(r.ActorSecret)
	write(r.SubjectSecret)

	return sum.Sum(nil)
}

// Genesis is what the first record of a chain is chained to.
var Genesis = make([]byte, sha256.Size)

// Valid reports whether an entry says enough to be recorded.
func (e Entry) valid() error {
	switch {
	case e.Action == "":
		return fmt.Errorf("an audit record needs an action")
	case e.Subject.Kind == "":
		return fmt.Errorf("an audit record needs the kind of thing acted on")
	case e.Actor.Kind == "":
		return fmt.Errorf("an audit record needs the kind of actor")
	case e.Actor.Kind != System && e.Actor.ID == "":
		return fmt.Errorf("an audit record of a %s needs their identifier", e.Actor.Kind)
	}
	return nil
}

// keyFor is whose key protects the state before and after.
//
// The person the state is about, when the caller named one; otherwise the
// person who acted, whose own work it describes; and the platform's own key
// when nobody personal was involved at all.
func (e Entry) keyFor() string {
	switch {
	case e.Subject.Person != "":
		return e.Subject.Person
	case e.Actor.ID != "":
		return e.Actor.ID
	default:
		return Platform
	}
}

// actorKey is whose key protects the origin address: the actor's own, or the
// platform's when the actor is the platform.
func (e Entry) actorKey() string {
	if e.Actor.ID != "" {
		return e.Actor.ID
	}
	return Platform
}
