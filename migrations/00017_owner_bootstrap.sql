-- The owner, and the first run that creates them
-- (docs/superpowers/specs/2026-09-26-f15-console-design.md, D2, D7).
--
-- "An owner exists" is said the way the rest of F15 will say who may do what:
-- a role, and its assignment to a staff account. The owner role is the one
-- row of `role` with `owner` true, assigned with the platform as its scope
-- (`marketplace_id` NULL). The permissions, the other roles and the
-- protections of the owner role — the trigger that refuses to delete or
-- change it, strip it or reassign it — arrive with the registry in F15's
-- second pull request, on these same tables; this one holds only what the
-- first run writes.
--
-- The three tables belong to the platform, not to a marketplace: only a
-- transaction that names no marketplace reads or writes them, which is the
-- console's (internal/staff). A marketplace's transaction sees no row of
-- them at all.

-- +goose Up
CREATE TABLE role (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    -- What the owner called the role. The owner role's is shown from the
    -- locale files instead, in the reader's language.
    name       text        NOT NULL,
    owner      boolean     NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now()
);
-- One owner role at most (D7).
CREATE UNIQUE INDEX role_one_owner ON role (owner) WHERE owner;

CREATE TABLE user_role (
    account_id     uuid        NOT NULL REFERENCES account (id) ON DELETE CASCADE,
    role_id        uuid        NOT NULL REFERENCES role (id) ON DELETE RESTRICT,
    -- The scope: one marketplace, or NULL for the whole platform.
    marketplace_id uuid REFERENCES marketplace (id) ON DELETE CASCADE,
    -- Who assigned it; NULL for the first run, which nobody signed in did.
    assigned_by    uuid REFERENCES account (id) ON DELETE SET NULL,
    assigned_at    timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT user_role_is_unique UNIQUE NULLS NOT DISTINCT (account_id, role_id, marketplace_id)
);
CREATE INDEX user_role_by_role ON user_role (role_id);

-- The first run, once. At most one row: `singleton` can only be true and is
-- the key, so a second first run conflicts with the first and waits for it.
-- The token is kept as its SHA-256 only (D2).
CREATE TABLE bootstrap (
    singleton  boolean     PRIMARY KEY DEFAULT true,
    token_hash bytea       NOT NULL,
    used_at    timestamptz NOT NULL,
    owner_id   uuid        NOT NULL REFERENCES account (id) ON DELETE RESTRICT,

    CONSTRAINT bootstrap_is_one_row CHECK (singleton)
);

ALTER TABLE role ENABLE ROW LEVEL SECURITY;
ALTER TABLE user_role ENABLE ROW LEVEL SECURITY;
ALTER TABLE bootstrap ENABLE ROW LEVEL SECURITY;

CREATE POLICY role_belongs_to_the_platform ON role
    FOR ALL USING (current_marketplace_id() IS NULL)
    WITH CHECK (current_marketplace_id() IS NULL);
CREATE POLICY user_role_belongs_to_the_platform ON user_role
    FOR ALL USING (current_marketplace_id() IS NULL)
    WITH CHECK (current_marketplace_id() IS NULL);
CREATE POLICY bootstrap_belongs_to_the_platform ON bootstrap
    FOR ALL USING (current_marketplace_id() IS NULL)
    WITH CHECK (current_marketplace_id() IS NULL);

-- +goose Down
DROP TABLE IF EXISTS bootstrap;
DROP TABLE IF EXISTS user_role;
DROP TABLE IF EXISTS role;
