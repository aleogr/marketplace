-- The sweep (Task 12b) deletes session rows that can never authenticate
-- again, once an hour, scoped by row-level security to the marketplace that
-- just signed in (marketplace_id = current_marketplace_id()). session has no
-- index on marketplace_id: token_hash is indexed by its UNIQUE constraint,
-- and session_by_account (migration 00011) only covers account_id where
-- revoked_at IS NULL. Without one, that DELETE's WHERE — the RLS predicate
-- ANDed with an OR of three timestamp comparisons — has nothing to narrow
-- the scan with, so it reads every session row of every marketplace, not
-- just the one being swept, and that cost grows with the whole table rather
-- than with one marketplace's sessions.

-- +goose Up
CREATE INDEX session_by_marketplace ON session (marketplace_id);

-- +goose Down
DROP INDEX IF EXISTS session_by_marketplace;
