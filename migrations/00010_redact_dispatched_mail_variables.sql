-- Clears an e-mail event's Variables from the outbox once nothing reads them
-- from the table again (spec, D6; migrations/00006_outbox_and_jobs.sql).
--
-- mail.Request writes a message's Variables into outbox_event.payload, and
-- for verify-email that includes the raw confirmation token, in the link
-- (internal/identity, migrations/00009_accounts.sql). The dispatcher hands
-- an event to its queue from what it already read into memory, never by
-- reading the row a second time, and Cloud Tasks carries that same snapshot
-- in the task body it calls back with (internal/platform/tasks) — so once an
-- event is dispatched, or parked because nothing more will retry it, its
-- row's own Variables serve no reader and a stolen database dump should not
-- still hold the token.
--
-- Scoped to kind = 'email.send', the only kind whose payload holds a
-- Variables object built from text that may carry a secret; email.event and
-- every other kind are untouched, and every reader of their payload reads it
-- from the in-memory event a consumer was handed, never from this table
-- after the fact (internal/platform/mail/brevo, internal/platform/mail).

-- +goose Up

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION outbox_dispatched(event uuid) RETURNS void
    LANGUAGE sql
    VOLATILE
    SECURITY DEFINER
    SET search_path = public, pg_temp
AS $$
    UPDATE outbox_event
    SET state = 'dispatched',
        dispatched_at = now(),
        attempts = attempts + 1,
        payload = CASE WHEN kind = 'email.send' THEN payload - 'Variables' ELSE payload END
    WHERE id = event
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION outbox_failed(event uuid, reason text, allowed integer) RETURNS text
    LANGUAGE sql
    VOLATILE
    SECURITY DEFINER
    SET search_path = public, pg_temp
AS $$
    UPDATE outbox_event
    SET attempts = attempts + 1,
        last_error = reason,
        state = CASE WHEN attempts + 1 >= allowed THEN 'parked' ELSE 'pending' END,
        payload = CASE WHEN attempts + 1 >= allowed AND kind = 'email.send'
                       THEN payload - 'Variables' ELSE payload END
    WHERE id = event
    RETURNING state
$$;
-- +goose StatementEnd

-- +goose Down

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION outbox_dispatched(event uuid) RETURNS void
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

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION outbox_failed(event uuid, reason text, allowed integer) RETURNS text
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
