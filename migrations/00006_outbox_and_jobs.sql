-- Work that must not happen inside a request.
--
-- Cloud Run scales to zero, so there is no always-on worker to hand work to
-- (docs/design.md, section 2.5). The pattern is an outbox: an operation writes
-- its events in the same transaction as the data that caused them, so the two
-- cannot disagree — a rolled-back sale sends no confirmation, and a committed
-- one always has its event waiting. A dispatcher then hands each event to
-- Cloud Tasks, which calls back into this same service.
--
-- What this table is not: a queue. Cloud Tasks is the queue, with the retries
-- and the backoff. This is the record of what happened and what has been handed
-- over, which is what makes the handover survive a crash between the two.

-- +goose Up

CREATE TABLE outbox_event (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    -- The marketplace the event belongs to, or null for the platform's own
    -- work. Tenant data is under row-level security below; the dispatcher
    -- reads through a function, because it serves every marketplace at once
    -- and belongs to none (migrations/00004_row_level_security.sql).
    marketplace_id uuid REFERENCES marketplace (id) ON DELETE CASCADE,

    -- What happened, as the consumers know it: `order.paid`, `label.bought`.
    kind           text        NOT NULL,
    -- Which queue carries it. One per class of work, because a flood of
    -- notifications must not delay a payment webhook (docs/design.md, 2.5).
    queue          text        NOT NULL,
    payload        jsonb       NOT NULL DEFAULT '{}'::jsonb,

    state          text        NOT NULL DEFAULT 'pending',
    attempts       integer     NOT NULL DEFAULT 0,
    -- Why the last attempt failed, for the person looking at a parked event.
    last_error     text,

    created_at     timestamptz NOT NULL DEFAULT now(),
    -- When it was handed to the queue, which is not when it was consumed.
    dispatched_at  timestamptz,

    CONSTRAINT outbox_event_state_is_known
        CHECK (state IN ('pending', 'dispatched', 'parked')),
    CONSTRAINT outbox_event_queue_is_known
        CHECK (queue IN ('webhooks', 'notifications', 'jobs'))
);

-- The dispatcher's own query: the oldest events still waiting.
CREATE INDEX outbox_event_pending
    ON outbox_event (created_at)
    WHERE state = 'pending';

ALTER TABLE outbox_event ENABLE ROW LEVEL SECURITY;

-- `IS NOT DISTINCT FROM`, not `=`: an event of the platform's own work carries
-- no marketplace, and null = null is null, which would refuse to write the
-- rows nothing owns. Read this way, a transaction serving a marketplace sees
-- that marketplace's events, and a transaction serving none sees the
-- platform's — never everyone's, which is the point of the policy.
CREATE POLICY outbox_event_belongs_to_the_marketplace ON outbox_event
    FOR ALL
    USING (marketplace_id IS NOT DISTINCT FROM current_marketplace_id())
    WITH CHECK (marketplace_id IS NOT DISTINCT FROM current_marketplace_id());

-- What a consumer has already done.
--
-- Cloud Tasks delivers at least once: a callback that succeeded but whose
-- answer was lost arrives again. A consumer that charged a card the first time
-- must not charge it again, so every delivery records itself here first, and a
-- second delivery of the same event to the same consumer finds the row and
-- stops.
CREATE TABLE outbox_delivery (
    event_id   uuid        NOT NULL REFERENCES outbox_event (id) ON DELETE CASCADE,
    consumer   text        NOT NULL,
    handled_at timestamptz NOT NULL DEFAULT now(),

    PRIMARY KEY (event_id, consumer)
);

-- The lease a scheduled job holds while it runs.
--
-- Cloud Scheduler can fire a job whose previous run is still going — a slow
-- reconciliation, an instance that was cold. Two runs reconciling the same
-- shipments would both write. The lease is what makes the second one stand
-- down, and it expires so that an instance killed mid-job does not hold the
-- job forever (docs/design.md, section 2.5).
--
-- It carries no `marketplace_id`: a job runs for the platform and works
-- across every marketplace, which is why it is not under row-level security.
CREATE TABLE job_lock (
    name         text        PRIMARY KEY,
    -- Who holds it, for the log: an instance id, so a stuck job can be traced
    -- to the instance that was running it.
    holder       text        NOT NULL,
    leased_until timestamptz NOT NULL,
    -- When the job last finished, which is what a person wants to know when
    -- they ask whether the job is running at all.
    finished_at  timestamptz
);

-- The dispatcher's way in.
--
-- It hands every marketplace's events to the queue and belongs to none, so it
-- cannot be scoped by a marketplace the way a request is. Rather than leave
-- the table unprotected, it reads and marks through these two functions, which
-- run as the owning role — the same single, named exception host resolution
-- uses (migrations/00004_row_level_security.sql).
--
-- The rows are locked and skipped: two instances dispatching at once take
-- different events rather than the same one twice.
-- +goose StatementBegin
CREATE FUNCTION outbox_pending(take integer)
    RETURNS TABLE (
        id             text,
        marketplace_id text,
        kind           text,
        queue          text,
        payload        jsonb,
        attempts       integer
    )
    LANGUAGE sql
    VOLATILE
    SECURITY DEFINER
    SET search_path = public, pg_temp
AS $$
    SELECT e.id::text, e.marketplace_id::text, e.kind, e.queue, e.payload, e.attempts
    FROM outbox_event e
    WHERE e.state = 'pending'
    ORDER BY e.created_at
    LIMIT take
    FOR UPDATE SKIP LOCKED
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION outbox_dispatched(event uuid) RETURNS void
    LANGUAGE sql
    VOLATILE
    SECURITY DEFINER
    SET search_path = public, pg_temp
AS $$
    UPDATE outbox_event
    SET state = 'dispatched', dispatched_at = now(), attempts = attempts + 1
    WHERE id = event
$$;
-- +goose StatementEnd

-- An event that could not be handed over. It is tried again while there are
-- attempts left, and parked after that: an event retried forever is an event
-- nobody ever looks at.
-- +goose StatementBegin
CREATE FUNCTION outbox_failed(event uuid, reason text, allowed integer) RETURNS text
    LANGUAGE sql
    VOLATILE
    SECURITY DEFINER
    SET search_path = public, pg_temp
AS $$
    UPDATE outbox_event
    SET attempts = attempts + 1,
        last_error = reason,
        state = CASE WHEN attempts + 1 >= allowed THEN 'parked' ELSE 'pending' END
    WHERE id = event
    RETURNING state
$$;
-- +goose StatementEnd

REVOKE ALL ON FUNCTION outbox_pending(integer) FROM PUBLIC;
REVOKE ALL ON FUNCTION outbox_dispatched(uuid) FROM PUBLIC;
REVOKE ALL ON FUNCTION outbox_failed(uuid, text, integer) FROM PUBLIC;

-- +goose Down

DROP FUNCTION IF EXISTS outbox_failed(uuid, text, integer);
DROP FUNCTION IF EXISTS outbox_dispatched(uuid);
DROP FUNCTION IF EXISTS outbox_pending(integer);
DROP TABLE job_lock;
DROP TABLE outbox_delivery;
DROP POLICY IF EXISTS outbox_event_belongs_to_the_marketplace ON outbox_event;
DROP TABLE outbox_event;
