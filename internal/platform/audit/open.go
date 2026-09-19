package audit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/aleogr/marketplace/internal/platform/keys"
)

// Opened is a record read back, with what could be opened opened.
type Opened struct {
	Entry
	Sequence int64
	// OriginErased and StateErased say that the part was written and its
	// person's key has since been destroyed. The record still proves an
	// operation happened, by whom and to what; what it said about the person
	// is gone, which is what was asked for (docs/requirements.md, section
	// 18.3).
	OriginErased bool
	StateErased  bool
}

// Open reads one record and decrypts what the keys still allow.
//
// It runs in a transaction scoped to the marketplace whose record it is, like
// every other read of tenant data: a marketplace's log is its own.
func (l *Log) Open(ctx context.Context, tx pgx.Tx, marketplace string, sequence int64) (Opened, error) {
	var found Opened
	var actor, subject, actorKey, subjectKey *string
	var actorSecret, subjectSecret []byte

	err := tx.QueryRow(ctx, `
		SELECT actor_id::text, actor_kind, action, subject_kind, subject_id,
		       actor_secret, actor_key::text, subject_secret, subject_key::text
		  FROM audit_log
		 WHERE marketplace_id IS NOT DISTINCT FROM $1 AND sequence = $2
	`, nullable(marketplace), sequence).Scan(
		&actor, &found.Actor.Kind, &found.Action, &found.Subject.Kind, &subject,
		&actorSecret, &actorKey, &subjectSecret, &subjectKey)
	if err != nil {
		return Opened{}, fmt.Errorf("cannot read audit record %d: %w", sequence, err)
	}

	found.Marketplace, found.Sequence = marketplace, sequence
	if actor != nil {
		found.Actor.ID = *actor
	}
	if subject != nil {
		found.Subject.ID = *subject
	}

	if len(actorSecret) > 0 {
		var where origin
		erased, err := l.open(ctx, tx, *actorKey, actorSecret, &where)
		if err != nil {
			return Opened{}, err
		}
		found.OriginErased, found.From = erased, where.From
	}

	if len(subjectSecret) > 0 {
		var changed state
		erased, err := l.open(ctx, tx, *subjectKey, subjectSecret, &changed)
		if err != nil {
			return Opened{}, err
		}
		found.StateErased = erased
		found.Before, found.After = changed.Before, changed.After
	}
	return found, nil
}

// open decrypts one part into content, and reports whether the key is gone.
func (l *Log) open(ctx context.Context, tx pgx.Tx, user string, sealed []byte, content any) (bool, error) {
	key, err := l.keys.open(ctx, tx, user)
	if errors.Is(err, keys.ErrDestroyed) {
		return true, nil
	}
	if err != nil {
		return false, err
	}

	plaintext, err := keys.Open(key, sealed)
	if err != nil {
		return false, fmt.Errorf("cannot open what the record holds: %w", err)
	}
	if err := json.Unmarshal(plaintext, content); err != nil {
		return false, fmt.Errorf("what the record holds is not readable: %w", err)
	}
	return false, nil
}
