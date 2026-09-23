-- Accounts, their passwords and the confirmation of their e-mail addresses
-- (docs/superpowers/specs/2026-09-23-f13-identity-core-design.md).
--
-- `account` and not `user`: `user` is a reserved word in PostgreSQL and would
-- need quoting in every query.
--
-- An account belongs to a marketplace (docs/requirements.md, section 4); a
-- staff account belongs to the platform and has no marketplace, which makes it
-- invisible to the application role under the policies below — staff access
-- arrives in F15 by its own path.
--
-- A correction to migrations/00008_audit_log.sql, which calls a person "the
-- platform's, not a marketplace's": that predates this table. `user_key.user_id`
-- holds an `account.id`, unique across the platform, so nothing about the keys
-- changes.

-- +goose Up
CREATE TABLE account (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    marketplace_id   uuid REFERENCES marketplace (id) ON DELETE RESTRICT,
    kind             text        NOT NULL,
    email            text        NOT NULL,
    email_normalised text        NOT NULL,
    name             text        NOT NULL,
    verified_at      timestamptz,
    created_at       timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT account_kind_is_known CHECK (kind IN ('buyer', 'staff')),
    CONSTRAINT account_staff_has_no_marketplace CHECK ((kind = 'staff') = (marketplace_id IS NULL)),
    -- NULLS NOT DISTINCT, so two staff accounts cannot share an address either.
    CONSTRAINT account_email_is_unique UNIQUE NULLS NOT DISTINCT (marketplace_id, email_normalised)
);

CREATE TABLE credential (
    account_id     uuid NOT NULL REFERENCES account (id) ON DELETE CASCADE,
    marketplace_id uuid REFERENCES marketplace (id) ON DELETE RESTRICT,
    kind           text        NOT NULL,
    -- The PHC string: the parameters travel with the hash (spec, D5).
    secret         text        NOT NULL,
    updated_at     timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (account_id, kind),
    CONSTRAINT credential_kind_is_known CHECK (kind IN ('password'))
);

CREATE TABLE email_verification (
    -- SHA-256 of the token. The raw token exists only in the e-mail and,
    -- until the message is dispatched or parked, in the outbox event that
    -- carries it (migrations/00010_redact_dispatched_mail_variables.sql).
    token_hash     bytea PRIMARY KEY,
    account_id     uuid NOT NULL REFERENCES account (id) ON DELETE CASCADE,
    marketplace_id uuid REFERENCES marketplace (id) ON DELETE RESTRICT,
    expires_at     timestamptz NOT NULL,
    used_at        timestamptz,
    created_at     timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX email_verification_by_account ON email_verification (account_id);

ALTER TABLE account ENABLE ROW LEVEL SECURITY;
ALTER TABLE credential ENABLE ROW LEVEL SECURITY;
ALTER TABLE email_verification ENABLE ROW LEVEL SECURITY;

CREATE POLICY account_belongs_to_the_marketplace ON account
    FOR ALL USING (marketplace_id = current_marketplace_id())
    WITH CHECK (marketplace_id = current_marketplace_id());
CREATE POLICY credential_belongs_to_the_marketplace ON credential
    FOR ALL USING (marketplace_id = current_marketplace_id())
    WITH CHECK (marketplace_id = current_marketplace_id());
CREATE POLICY email_verification_belongs_to_the_marketplace ON email_verification
    FOR ALL USING (marketplace_id = current_marketplace_id())
    WITH CHECK (marketplace_id = current_marketplace_id());

-- +goose Down
DROP TABLE IF EXISTS email_verification;
DROP TABLE IF EXISTS credential;
DROP TABLE IF EXISTS account;
