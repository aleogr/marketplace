package audit

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/aleogr/marketplace/internal/platform/keys"
)

// Keys is one data key per person, wrapped by a keeper.
//
// Nothing is cached. A key held in memory would keep working after it was
// destroyed, for as long as the instance lived — and "it stopped working
// eventually" is not what a deletion request was promised (docs/requirements.md,
// section 18.3). The cost is a call to the keeper per record, which is what the
// promise costs.
type Keys struct{ keeper keys.Keeper }

// NewKeys returns the keys, wrapped by keeper.
func NewKeys(keeper keys.Keeper) *Keys { return &Keys{keeper: keeper} }

// Keeper reports which keeper protects these keys.
func (k *Keys) Keeper() string { return k.keeper.Name() }

// open returns a person's data key, creating one the first time.
//
// It runs in the caller's transaction, so a key created for a record that is
// then rolled back is rolled back with it.
func (k *Keys) open(ctx context.Context, tx pgx.Tx, user string) ([]byte, error) {
	wrapped, err := k.read(ctx, tx, user)
	if errors.Is(err, pgx.ErrNoRows) {
		if wrapped, err = k.create(ctx, tx, user); err != nil {
			return nil, err
		}
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	key, err := k.keeper.Unwrap(ctx, wrapped)
	if err != nil {
		return nil, fmt.Errorf("cannot open the key of %s: %w", user, err)
	}
	return key, nil
}

// read returns the wrapped key, or says it was destroyed.
func (k *Keys) read(ctx context.Context, tx pgx.Tx, user string) ([]byte, error) {
	var wrapped []byte
	var destroyed *string
	err := tx.QueryRow(ctx,
		`SELECT wrapped_key, destroyed_at::text FROM user_key WHERE user_id = $1`,
		user).Scan(&wrapped, &destroyed)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return nil, pgx.ErrNoRows
	case err != nil:
		return nil, fmt.Errorf("cannot read the key of %s: %w", user, err)
	case destroyed != nil:
		return nil, fmt.Errorf("%w: %s, on %s", keys.ErrDestroyed, user, *destroyed)
	}
	return wrapped, nil
}

// create makes a key for a person who had none.
func (k *Keys) create(ctx context.Context, tx pgx.Tx, user string) ([]byte, error) {
	key, err := keys.New()
	if err != nil {
		return nil, err
	}

	wrapped, err := k.keeper.Wrap(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("cannot wrap the key of %s: %w", user, err)
	}

	// Two writers may reach this at once for the same person; the one that
	// loses reads the winner's key rather than overwriting it, because
	// overwriting would make every earlier record of that person unreadable.
	tag, err := tx.Exec(ctx, `
		INSERT INTO user_key (user_id, wrapped_key, keeper)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id) DO NOTHING
	`, user, wrapped, k.keeper.Name())
	if err != nil {
		return nil, fmt.Errorf("cannot store the key of %s: %w", user, err)
	}
	if tag.RowsAffected() == 1 {
		return wrapped, nil
	}
	return k.read(ctx, tx, user)
}

// Destroy erases a person's key, and with it everything the log says about
// them.
//
// The records stay where they are and the chain still verifies: what changes is
// that nothing can open what they hold (docs/requirements.md, section 21).
func (k *Keys) Destroy(ctx context.Context, tx pgx.Tx, user string) error {
	if user == Platform {
		// There is nobody to ask for it, and destroying it would make the
		// platform's own history unreadable without erasing anybody's personal
		// data.
		return errors.New("the platform's own key is not a person's to destroy")
	}

	_, err := tx.Exec(ctx, `
		UPDATE user_key
		   SET wrapped_key = NULL, destroyed_at = now()
		 WHERE user_id = $1 AND destroyed_at IS NULL
	`, user)
	if err != nil {
		return fmt.Errorf("cannot destroy the key of %s: %w", user, err)
	}
	return nil
}
