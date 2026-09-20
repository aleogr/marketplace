-- The audit log: who did what, when, from where, and what changed.
--
-- Three properties, and each of them is a decision that cannot be added later
-- (docs/requirements.md, section 21).
--
-- **Append-only, twice over.** The application role is granted everything on
-- every table by default, so the grant that runs after these migrations takes
-- `UPDATE` and `DELETE` back on this one (internal/platform/db/migrate.go), and
-- a trigger refuses them whatever role is connected. Permissions are what stop
-- the application; the trigger is what stops the day somebody restores a
-- backup with different ones.
--
-- **Tamper-evident.** Each record carries the hash of the one before it, so
-- altering a record out of band breaks every hash after it. The chain is kept
-- per marketplace, plus one for the platform: a single chain for everything
-- would make every write in the system queue behind one row.
--
-- **Erasable without a hole.** A record names people by internal identifier and
-- nothing else. What inherently carries personal data — the origin address, the
-- state before and after — is encrypted, and the key belongs to the person the
-- content is about. A deletion request destroys that key: the record stays, the
-- chain still verifies, and the content is gone (sections 21 and 18.3).

-- +goose Up

-- One key per person, wrapped by a key this database never holds.
--
-- Deliberately outside row-level security and carrying no marketplace: a person
-- is the platform's, not a marketplace's, and the same person acts in more than
-- one. What a marketplace may read is its own log, and that is where the policy
-- belongs.
CREATE TABLE user_key (
    -- The internal identifier, and the only thing about the person this table
    -- knows. Users arrive in F13; until then this references nothing, on
    -- purpose — an audit log that waits for the identity model is an audit log
    -- that misses the deliveries before it.
    user_id      uuid PRIMARY KEY,
    -- The data key, encrypted by the keeper. Null once destroyed.
    wrapped_key  bytea,
    -- Which keeper wrapped it. A key wrapped by one cannot be unwrapped by
    -- another, and knowing which is which is the difference between a
    -- migration and a loss.
    keeper       text        NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    destroyed_at timestamptz,

    CONSTRAINT user_key_is_destroyed_or_present
        CHECK ((wrapped_key IS NULL) = (destroyed_at IS NOT NULL))
);

-- The head of each chain, and the row a writer locks to append.
--
-- The platform's own chain is the row whose marketplace is null. A primary key
-- cannot be nullable, so uniqueness is an index over the null-collapsed value.
CREATE TABLE audit_chain (
    marketplace_id uuid REFERENCES marketplace (id) ON DELETE RESTRICT,
    -- The hash of the last record, and how many there are. The empty chain
    -- starts at 32 zero bytes, which is what the first record is chained to.
    last_hash      bytea       NOT NULL DEFAULT '\x0000000000000000000000000000000000000000000000000000000000000000'::bytea,
    length         bigint      NOT NULL DEFAULT 0,
    updated_at     timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT audit_chain_hash_is_sha256 CHECK (octet_length(last_hash) = 32)
);

CREATE UNIQUE INDEX audit_chain_one_per_marketplace
    ON audit_chain ((COALESCE(marketplace_id, '00000000-0000-0000-0000-000000000000'::uuid)));

CREATE TABLE audit_log (
    id             bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    -- Null for the platform's own operations, which is also its own chain.
    marketplace_id uuid REFERENCES marketplace (id) ON DELETE RESTRICT,
    -- Where this record sits in its chain, from 1.
    sequence       bigint      NOT NULL,
    recorded_at    timestamptz NOT NULL DEFAULT now(),

    -- Who. An internal identifier and a kind, never a name, an address or a
    -- document (section 21).
    actor_id       uuid,
    actor_kind     text        NOT NULL,

    -- What, and to what. The subject's identifier is text because not every
    -- subject is identified by a uuid — a parameter has a name.
    action         text        NOT NULL,
    subject_kind   text        NOT NULL,
    subject_id     text,

    -- From where, encrypted with the **actor's** key: the origin address is
    -- the actor's personal data, and it is their deletion request that must
    -- make it unreadable.
    actor_secret   bytea,
    actor_key      uuid,

    -- The state before and after, encrypted with the **subject's** key when
    -- the subject is a person: that content is about them, and staff editing a
    -- buyer's record must not leave the buyer's data behind the staff
    -- member's key. Two envelopes, because one would make one of the two
    -- promises false.
    subject_secret bytea,
    subject_key    uuid,

    -- The chain.
    previous_hash  bytea       NOT NULL,
    hash           bytea       NOT NULL,

    CONSTRAINT audit_log_actor_kind_is_known
        CHECK (actor_kind IN ('staff', 'store', 'buyer', 'system')),
    CONSTRAINT audit_log_sequence_is_positive CHECK (sequence > 0),
    CONSTRAINT audit_log_hashes_are_sha256
        CHECK (octet_length(hash) = 32 AND octet_length(previous_hash) = 32),
    CONSTRAINT audit_log_secrets_name_their_key
        CHECK ((actor_secret IS NULL) = (actor_key IS NULL)
           AND (subject_secret IS NULL) = (subject_key IS NULL)),

    -- One record per position per chain. `NULLS NOT DISTINCT` is what makes
    -- that true of the platform's chain too, where the marketplace is null.
    UNIQUE NULLS NOT DISTINCT (marketplace_id, sequence)
);

CREATE INDEX audit_log_by_chain ON audit_log (marketplace_id, sequence);
CREATE INDEX audit_log_by_actor ON audit_log (actor_id, recorded_at DESC);
CREATE INDEX audit_log_by_subject ON audit_log (subject_kind, subject_id, recorded_at DESC);

-- The second belt. The grant after these migrations takes UPDATE and DELETE
-- away from the application role; this refuses them to every role, including
-- the one that owns the table and the one a restored backup runs as.
-- +goose StatementBegin
CREATE FUNCTION audit_log_is_append_only() RETURNS trigger
    LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION 'audit_log is append-only: % is not allowed', TG_OP
        USING ERRCODE = 'restrict_violation';
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER audit_log_refuses_changes
    BEFORE UPDATE OR DELETE ON audit_log
    FOR EACH ROW EXECUTE FUNCTION audit_log_is_append_only();

CREATE TRIGGER audit_log_refuses_truncation
    BEFORE TRUNCATE ON audit_log
    FOR EACH STATEMENT EXECUTE FUNCTION audit_log_is_append_only();

-- A marketplace reads its own log and nothing else, by the same rule as every
-- other tenant table (docs/design.md, section 2.6). The platform's records are
-- the ones belonging to no marketplace, which a transaction that named none
-- sees.
ALTER TABLE audit_log ENABLE ROW LEVEL SECURITY;
ALTER TABLE audit_chain ENABLE ROW LEVEL SECURITY;

CREATE POLICY audit_log_belongs_to_the_marketplace ON audit_log
    FOR ALL
    USING (marketplace_id IS NOT DISTINCT FROM current_marketplace_id())
    WITH CHECK (marketplace_id IS NOT DISTINCT FROM current_marketplace_id());

CREATE POLICY audit_chain_belongs_to_the_marketplace ON audit_chain
    FOR ALL
    USING (marketplace_id IS NOT DISTINCT FROM current_marketplace_id())
    WITH CHECK (marketplace_id IS NOT DISTINCT FROM current_marketplace_id());

-- Verification reads every chain, which is exactly what the policy above
-- forbids. It is the same escape hatch host resolution uses: a function that
-- runs as the owner, does one thing, and is granted to the application role by
-- name (migrations/00004_row_level_security.sql).
-- +goose StatementBegin
CREATE FUNCTION audit_chains()
    RETURNS TABLE (marketplace_id text, last_hash bytea, length bigint)
    LANGUAGE sql
    STABLE
    SECURITY DEFINER
    SET search_path = public, pg_temp
AS $$
    SELECT c.marketplace_id::text, c.last_hash, c.length
    FROM audit_chain c
    ORDER BY c.marketplace_id NULLS FIRST
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION audit_records(chain uuid, after bigint, take integer)
    RETURNS TABLE (
        sequence       bigint,
        recorded_at    timestamptz,
        actor_id       text,
        actor_kind     text,
        action         text,
        subject_kind   text,
        subject_id     text,
        actor_secret   bytea,
        subject_secret bytea,
        previous_hash  bytea,
        hash           bytea
    )
    LANGUAGE sql
    STABLE
    SECURITY DEFINER
    SET search_path = public, pg_temp
AS $$
    SELECT l.sequence, l.recorded_at, l.actor_id::text, l.actor_kind, l.action,
           l.subject_kind, l.subject_id, l.actor_secret, l.subject_secret,
           l.previous_hash, l.hash
    FROM audit_log l
    WHERE l.marketplace_id IS NOT DISTINCT FROM chain
      AND l.sequence > after
    ORDER BY l.sequence
    LIMIT take
$$;
-- +goose StatementEnd

REVOKE ALL ON FUNCTION audit_chains() FROM PUBLIC;
REVOKE ALL ON FUNCTION audit_records(uuid, bigint, integer) FROM PUBLIC;

-- +goose Down

DROP FUNCTION IF EXISTS audit_records(uuid, bigint, integer);
DROP FUNCTION IF EXISTS audit_chains();
DROP POLICY IF EXISTS audit_chain_belongs_to_the_marketplace ON audit_chain;
DROP POLICY IF EXISTS audit_log_belongs_to_the_marketplace ON audit_log;
DROP TRIGGER IF EXISTS audit_log_refuses_truncation ON audit_log;
DROP TRIGGER IF EXISTS audit_log_refuses_changes ON audit_log;
DROP FUNCTION IF EXISTS audit_log_is_append_only();
DROP TABLE audit_log;
DROP TABLE audit_chain;
DROP TABLE user_key;
