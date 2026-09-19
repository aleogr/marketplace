package audit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/aleogr/marketplace/internal/platform/keys"
)

// Log appends records and verifies the chains they form.
type Log struct {
	keys *Keys
	log  *slog.Logger
}

// NewLog returns the audit log.
func NewLog(keys *Keys, log *slog.Logger) *Log { return &Log{keys: keys, log: log} }

// Append records one operation, in the transaction that performed it.
//
// A transaction rather than a connection of its own, and that is the whole
// design: an operation that rolls back leaves no record of having happened, and
// one that commits cannot commit without its record. The outbox is written the
// same way and for the same reason (internal/platform/outbox).
func (l *Log) Append(ctx context.Context, tx pgx.Tx, entry Entry) error {
	if err := entry.valid(); err != nil {
		return err
	}

	actorSecret, err := l.seal(ctx, tx, entry.actorKey(), origin{From: entry.From},
		entry.From != "")
	if err != nil {
		return err
	}

	carries := len(entry.Before) > 0 || len(entry.After) > 0
	subjectSecret, err := l.seal(ctx, tx, entry.keyFor(),
		state{Before: entry.Before, After: entry.After}, carries)
	if err != nil {
		return err
	}

	// The chain's head, locked. Two operations in the same marketplace append
	// one after the other; two in different marketplaces do not wait for each
	// other at all, which is why the chains are per marketplace
	// (docs/roadmap.md, F12).
	last, length, err := l.head(ctx, tx, entry.Marketplace)
	if err != nil {
		return err
	}

	written := record{
		Marketplace: entry.Marketplace,
		Sequence:    length + 1,
		// Truncated to what the database keeps, so that the value hashed here
		// and the value stored there are the same value.
		RecordedAt:    time.Now().UTC().Truncate(time.Microsecond),
		ActorID:       entry.Actor.ID,
		ActorKind:     string(entry.Actor.Kind),
		Action:        entry.Action,
		SubjectKind:   entry.Subject.Kind,
		SubjectID:     entry.Subject.ID,
		ActorSecret:   actorSecret,
		SubjectSecret: subjectSecret,
		PreviousHash:  last,
	}
	written.Hash = digest(last, written)

	if err := l.insert(ctx, tx, entry, written); err != nil {
		return err
	}

	_, err = tx.Exec(ctx, `
		UPDATE audit_chain
		   SET last_hash = $2, length = $3, updated_at = now()
		 WHERE marketplace_id IS NOT DISTINCT FROM $1
	`, nullable(entry.Marketplace), written.Hash, written.Sequence)
	if err != nil {
		return fmt.Errorf("cannot advance the audit chain: %w", err)
	}
	return nil
}

// seal encrypts one part of a record with one person's key, or returns nothing
// when there is nothing to protect.
func (l *Log) seal(ctx context.Context, tx pgx.Tx, user string, content any, carries bool) ([]byte, error) {
	if !carries {
		return nil, nil
	}

	plaintext, err := json.Marshal(content)
	if err != nil {
		return nil, fmt.Errorf("cannot prepare the record's content: %w", err)
	}

	key, err := l.keys.open(ctx, tx, user)
	if err != nil {
		return nil, err
	}

	sealed, err := keys.Seal(key, plaintext)
	if err != nil {
		return nil, fmt.Errorf("cannot seal the record's content: %w", err)
	}
	return sealed, nil
}

// head returns the chain's last hash and length, with the row locked until the
// transaction ends.
func (l *Log) head(ctx context.Context, tx pgx.Tx, marketplace string) ([]byte, int64, error) {
	// The chain of a marketplace that has never been audited begins here.
	if _, err := tx.Exec(ctx, `
		INSERT INTO audit_chain (marketplace_id) VALUES ($1)
		ON CONFLICT ((COALESCE(marketplace_id, '00000000-0000-0000-0000-000000000000'::uuid)))
		DO NOTHING
	`, nullable(marketplace)); err != nil {
		return nil, 0, fmt.Errorf("cannot open the audit chain: %w", err)
	}

	var last []byte
	var length int64
	err := tx.QueryRow(ctx, `
		SELECT last_hash, length FROM audit_chain
		 WHERE marketplace_id IS NOT DISTINCT FROM $1
		   FOR UPDATE
	`, nullable(marketplace)).Scan(&last, &length)
	if err != nil {
		return nil, 0, fmt.Errorf("cannot read the audit chain: %w", err)
	}
	return last, length, nil
}

func (l *Log) insert(ctx context.Context, tx pgx.Tx, entry Entry, written record) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO audit_log
		    (marketplace_id, sequence, recorded_at, actor_id, actor_kind, action,
		     subject_kind, subject_id, actor_secret, actor_key,
		     subject_secret, subject_key, previous_hash, hash)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`,
		nullable(entry.Marketplace), written.Sequence, written.RecordedAt,
		nullable(written.ActorID), written.ActorKind, written.Action,
		written.SubjectKind, nullable(written.SubjectID),
		written.ActorSecret, keyOf(written.ActorSecret, entry.actorKey()),
		written.SubjectSecret, keyOf(written.SubjectSecret, entry.keyFor()),
		written.PreviousHash, written.Hash)
	if err != nil {
		return fmt.Errorf("cannot record %s: %w", entry.Action, err)
	}
	return nil
}

// Break is a record whose hash does not follow from the one before it.
type Break struct {
	Marketplace string
	Sequence    int64
	// Why says which of the two ways it is broken: the record's own hash does
	// not match its contents, or it is not chained to its predecessor.
	Why string
}

// Report is what one verification found.
type Report struct {
	Chains  int
	Records int64
	Breaks  []Break
}

// Reader is the part of the pool verification uses.
type Reader interface {
	InTx(ctx context.Context, fn func(pgx.Tx) error) error
}

// verifyBatch is how many records are read at a time. The log outgrows memory
// long before it outgrows the database.
const verifyBatch = 500

// Verify walks every chain and recomputes it.
//
// It reads through functions that run as the table's owner, because row-level
// security answers a transaction that named no marketplace with the platform's
// records alone — and what is being verified is every marketplace's
// (migrations/00008_audit_log.sql).
func (l *Log) Verify(ctx context.Context, db Reader) (Report, error) {
	var report Report

	chains, err := l.chains(ctx, db)
	if err != nil {
		return report, err
	}
	report.Chains = len(chains)

	for _, chain := range chains {
		previous := Genesis
		var after int64

		for {
			batch, err := l.records(ctx, db, chain, after, verifyBatch)
			if err != nil {
				return report, err
			}
			if len(batch) == 0 {
				break
			}

			for _, found := range batch {
				report.Records++
				after = found.Sequence

				switch {
				case !bytes.Equal(found.PreviousHash, previous):
					report.Breaks = append(report.Breaks, Break{
						Marketplace: chain, Sequence: found.Sequence,
						Why: "it is not chained to the record before it",
					})
				case !bytes.Equal(digest(previous, found), found.Hash):
					report.Breaks = append(report.Breaks, Break{
						Marketplace: chain, Sequence: found.Sequence,
						Why: "its hash does not match what it holds",
					})
				}

				// The chain continues from what the record says, not from what
				// it should have said: one altered record is one break, not a
				// break in everything after it.
				previous = found.Hash
			}
		}
	}

	if len(report.Breaks) > 0 {
		for _, broken := range report.Breaks {
			l.log.ErrorContext(ctx, "an audit record does not verify",
				"marketplace", broken.Marketplace, "sequence", broken.Sequence,
				"reason", broken.Why)
		}
	}
	return report, nil
}

func (l *Log) chains(ctx context.Context, db Reader) ([]string, error) {
	var found []string
	err := db.InTx(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT marketplace_id FROM audit_chains()`)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var marketplace *string
			if err := rows.Scan(&marketplace); err != nil {
				return err
			}
			if marketplace == nil {
				found = append(found, "")
				continue
			}
			found = append(found, *marketplace)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, fmt.Errorf("cannot read the audit chains: %w", err)
	}
	return found, nil
}

func (l *Log) records(ctx context.Context, db Reader, chain string, after int64, take int) ([]record, error) {
	var found []record
	err := db.InTx(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx,
			`SELECT sequence, recorded_at, actor_id, actor_kind, action,
			        subject_kind, subject_id, actor_secret, subject_secret,
			        previous_hash, hash
			   FROM audit_records($1, $2, $3)`,
			nullable(chain), after, take)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var one record
			var actor, subject *string
			if err := rows.Scan(&one.Sequence, &one.RecordedAt, &actor, &one.ActorKind,
				&one.Action, &one.SubjectKind, &subject, &one.ActorSecret,
				&one.SubjectSecret, &one.PreviousHash, &one.Hash); err != nil {
				return err
			}
			one.Marketplace = chain
			if actor != nil {
				one.ActorID = *actor
			}
			if subject != nil {
				one.SubjectID = *subject
			}
			// PostgreSQL hands the moment back in whatever zone the session
			// is in; the digest is over the instant, so it is read as one.
			one.RecordedAt = one.RecordedAt.UTC()
			found = append(found, one)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, fmt.Errorf("cannot read the records of a chain: %w", err)
	}
	return found, nil
}

// nullable turns an empty identifier into a NULL: the platform's own chain is
// the one belonging to no marketplace.
func nullable(value string) any {
	if value == "" {
		return nil
	}
	return value
}

// keyOf names whose key sealed a part, and nothing when the part is absent —
// the table refuses a key with no secret and a secret with no key.
func keyOf(sealed []byte, user string) any {
	if len(sealed) == 0 {
		return nil
	}
	return user
}
