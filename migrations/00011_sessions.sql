-- Sessions stored server-side and revocable from the first release
-- (docs/requirements.md, section 18.1). Only the token's SHA-256 is stored.

-- +goose Up
CREATE TABLE session (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id     uuid NOT NULL REFERENCES account (id) ON DELETE CASCADE,
    marketplace_id uuid REFERENCES marketplace (id) ON DELETE RESTRICT,
    token_hash     bytea       NOT NULL UNIQUE,
    created_at     timestamptz NOT NULL DEFAULT now(),
    last_seen_at   timestamptz NOT NULL DEFAULT now(),
    ip             text        NOT NULL,
    user_agent     text        NOT NULL,
    revoked_at     timestamptz,
    revoked_reason text,
    CONSTRAINT session_revocation_has_a_reason CHECK ((revoked_at IS NULL) = (revoked_reason IS NULL))
);
CREATE INDEX session_by_account ON session (account_id) WHERE revoked_at IS NULL;

ALTER TABLE session ENABLE ROW LEVEL SECURITY;
CREATE POLICY session_belongs_to_the_marketplace ON session
    FOR ALL USING (marketplace_id = current_marketplace_id())
    WITH CHECK (marketplace_id = current_marketplace_id());

-- +goose Down
DROP TABLE IF EXISTS session;
