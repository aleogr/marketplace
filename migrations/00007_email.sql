-- What the platform has sent, and to whom it must stop sending.
--
-- An address that hard-bounced or reported the message as spam is never
-- written to again (docs/requirements.md, section 17). That is not politeness:
-- a sender that keeps mailing dead addresses and complainers loses the
-- reputation that makes the rest of its mail arrive at all, and reputation is
-- shared by every marketplace on the domain.

-- +goose Up

CREATE TABLE email_suppression (
    -- Lower-cased by the application, because an address is not
    -- case-sensitive in the part that matters here and a suppression that
    -- missed `Name@example.com` would be no suppression at all.
    address    text PRIMARY KEY,
    -- `bounce` or `complaint`. What the provider called it is kept beside it,
    -- because providers name these differently and the original is what a
    -- person needs when they ask why an address stopped receiving.
    reason     text        NOT NULL,
    provider   text        NOT NULL,
    provider_event text,
    detail     text,
    created_at timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT email_suppression_reason_is_known
        CHECK (reason IN ('bounce', 'complaint'))
);

-- Every send that was attempted, and every one that was not.
--
-- A message that was skipped is as important as one that was sent: without
-- this row, an address that stopped receiving mail looks exactly like a
-- feature that never ran.
CREATE TABLE email_send (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    -- The marketplace whose mail this is, or null for the platform's own.
    marketplace_id uuid REFERENCES marketplace (id) ON DELETE CASCADE,
    address    text        NOT NULL,
    template   text        NOT NULL,
    language   text        NOT NULL,
    state      text        NOT NULL,
    -- The provider's own id for the message, which is what a support question
    -- is answered with ("what happened to the message I was promised").
    provider_message text,
    detail     text,
    created_at timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT email_send_state_is_known
        CHECK (state IN ('sent', 'skipped', 'failed'))
);

CREATE INDEX email_send_by_address ON email_send (address, created_at DESC);

ALTER TABLE email_send ENABLE ROW LEVEL SECURITY;

CREATE POLICY email_send_belongs_to_the_marketplace ON email_send
    FOR ALL
    USING (marketplace_id IS NOT DISTINCT FROM current_marketplace_id())
    WITH CHECK (marketplace_id IS NOT DISTINCT FROM current_marketplace_id());

-- The suppression list is deliberately not under row-level security and
-- carries no marketplace: an address that reported one marketplace's mail as
-- spam must not receive another's either. The reputation being protected
-- belongs to the sending domain, which is the platform's.

-- +goose Down

DROP POLICY IF EXISTS email_send_belongs_to_the_marketplace ON email_send;
DROP TABLE email_send;
DROP TABLE email_suppression;
