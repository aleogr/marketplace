package audit

import (
	"bytes"
	"testing"
	"time"
)

// entry returns a record with every field filled, so that a test changing one
// is changing exactly one.
func entry() record {
	return record{
		Marketplace:   "6f1c7b8e-0a6d-4d3a-9a0e-0b2f2a9a1b11",
		Sequence:      7,
		RecordedAt:    time.Date(2026, 9, 19, 8, 30, 0, 0, time.UTC),
		ActorID:       "2b0b7a1e-9d1e-4a5f-9b3d-6f7c8a9b0c1d",
		ActorKind:     string(Staff),
		Action:        "parameter.changed",
		SubjectKind:   "parameter",
		SubjectID:     "commission.rate",
		ActorSecret:   []byte("where from, sealed"),
		SubjectSecret: []byte("what changed, sealed"),
	}
}

func TestTheDigestIsTheSameForTheSameRecord(t *testing.T) {
	first := digest(Genesis, entry())
	second := digest(Genesis, entry())

	if !bytes.Equal(first, second) {
		t.Error("the same record hashed to two different values")
	}
}

// Everything the record holds is covered, including the ciphertexts: altering
// what a record says about somebody, even unreadably, must break the chain.
func TestEveryFieldChangesTheDigest(t *testing.T) {
	original := digest(Genesis, entry())

	changes := map[string]func(*record){
		"the marketplace":  func(r *record) { r.Marketplace = "another" },
		"the position":     func(r *record) { r.Sequence = 8 },
		"the moment":       func(r *record) { r.RecordedAt = r.RecordedAt.Add(time.Microsecond) },
		"who acted":        func(r *record) { r.ActorID = "somebody else" },
		"what kind of act": func(r *record) { r.ActorKind = string(System) },
		"the action":       func(r *record) { r.Action = "parameter.read" },
		"the subject kind": func(r *record) { r.SubjectKind = "order" },
		"the subject":      func(r *record) { r.SubjectID = "commission.cap" },
		"the origin":       func(r *record) { r.ActorSecret = []byte("somewhere else, sealed") },
		"the state":        func(r *record) { r.SubjectSecret = []byte("something else, sealed") },
	}

	for name, change := range changes {
		t.Run(name, func(t *testing.T) {
			altered := entry()
			change(&altered)

			if bytes.Equal(original, digest(Genesis, altered)) {
				t.Errorf("changing %s left the digest untouched", name)
			}
		})
	}
}

func TestTheDigestFollowsTheRecordBefore(t *testing.T) {
	first := digest(Genesis, entry())
	second := digest([]byte("a different predecessor.........."), entry())

	if bytes.Equal(first, second) {
		t.Error("the same record after two different records hashed the same")
	}
}

// Fields are hashed with their length in front, so that content cannot be
// moved across a boundary to produce the same bytes — the classic way a chain
// is defeated without altering what it holds.
func TestContentCannotBeMovedAcrossAFieldBoundary(t *testing.T) {
	left := entry()
	left.Action, left.SubjectKind = "parameter.chang", "ed"

	right := entry()
	right.Action, right.SubjectKind = "parameter.change", "d"

	if bytes.Equal(digest(Genesis, left), digest(Genesis, right)) {
		t.Error("two records hashed the same with a letter moved between fields")
	}
}

// A nanosecond is hashed as the microsecond the database keeps, because a
// digest the stored row cannot reproduce is a chain that never verifies.
func TestTheMomentIsHashedAtTheDatabaseResolution(t *testing.T) {
	coarse := entry()
	fine := entry()
	fine.RecordedAt = fine.RecordedAt.Add(400 * time.Nanosecond)

	if !bytes.Equal(digest(Genesis, coarse), digest(Genesis, fine)) {
		t.Error("nanoseconds changed the digest; the stored record could never match it")
	}
}

func TestAnEntryMustSayEnoughToBeRecorded(t *testing.T) {
	complete := Entry{
		Actor:   Actor{ID: "2b0b7a1e-9d1e-4a5f-9b3d-6f7c8a9b0c1d", Kind: Staff},
		Action:  "parameter.changed",
		Subject: Subject{Kind: "parameter", ID: "commission.rate"},
	}
	if err := complete.valid(); err != nil {
		t.Fatalf("a complete entry was refused: %v", err)
	}

	for name, change := range map[string]func(*Entry){
		"no action":           func(e *Entry) { e.Action = "" },
		"no subject":          func(e *Entry) { e.Subject.Kind = "" },
		"no kind of actor":    func(e *Entry) { e.Actor.Kind = "" },
		"a person with no id": func(e *Entry) { e.Actor.ID = "" },
	} {
		t.Run(name, func(t *testing.T) {
			incomplete := complete
			change(&incomplete)

			if err := incomplete.valid(); err == nil {
				t.Errorf("an entry with %s was accepted", name)
			}
		})
	}

	// The system has no identifier, and needs none: it is not a person.
	work := complete
	work.Actor = Actor{Kind: System}
	if err := work.valid(); err != nil {
		t.Errorf("the system's own work was refused: %v", err)
	}
}

// Whose key protects what: the person the state is about, then the person who
// acted, then the platform. The middle case is the one that matters — staff
// editing a buyer's record must leave the buyer's data behind the buyer's key,
// or the buyer's deletion request would not reach it.
func TestTheStateIsProtectedByThePersonItIsAbout(t *testing.T) {
	staff := Entry{Actor: Actor{ID: "the-staff", Kind: Staff}}

	for name, testCase := range map[string]struct {
		entry Entry
		want  string
	}{
		"the subject is a person": {
			entry: Entry{Actor: staff.Actor, Subject: Subject{Person: "the-buyer"}},
			want:  "the-buyer",
		},
		"the subject is not a person": {entry: staff, want: "the-staff"},
		"nobody personally acted": {
			entry: Entry{Actor: Actor{Kind: System}},
			want:  Platform,
		},
	} {
		t.Run(name, func(t *testing.T) {
			if got := testCase.entry.keyFor(); got != testCase.want {
				t.Errorf("the state is protected by %q, want %q", got, testCase.want)
			}
		})
	}
}

// And the origin address is always the actor's own, whoever the subject is.
func TestTheOriginIsProtectedByWhoeverActed(t *testing.T) {
	staff := Entry{
		Actor:   Actor{ID: "the-staff", Kind: Staff},
		Subject: Subject{Person: "the-buyer"},
	}
	if got := staff.actorKey(); got != "the-staff" {
		t.Errorf("the origin is protected by %q, want the actor's own key", got)
	}

	if got := (Entry{Actor: Actor{Kind: System}}).actorKey(); got != Platform {
		t.Errorf("the platform's own origin is protected by %q, want %q", got, Platform)
	}
}
