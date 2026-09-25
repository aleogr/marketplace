-- The second step of a sign-in, the step-up, and the codes sent by e-mail
-- (docs/superpowers/specs/2026-09-25-f14-two-factor-design.md, D3, D4, D6).

-- +goose Up

-- A correct password on an account with a second factor opens a challenge,
-- not a session (D3): the session table keeps holding complete sessions only.
-- A step-up opens one too, naming the session it steps up and the action it
-- guards. Each is found by the SHA-256 of a token in its own cookie, lives
-- five minutes, takes five attempts and is used once.
CREATE TABLE sign_in_challenge (
    token_hash       bytea PRIMARY KEY,
    account_id       uuid NOT NULL REFERENCES account (id) ON DELETE CASCADE,
    marketplace_id   uuid REFERENCES marketplace (id) ON DELETE RESTRICT,
    session_id       uuid REFERENCES session (id) ON DELETE CASCADE,
    action           text,
    expires_at       timestamptz NOT NULL,
    attempts         integer     NOT NULL DEFAULT 0,
    used_at          timestamptz,
    -- The WebAuthn assertion's state: its challenge, which the browser's
    -- answer must carry.
    webauthn_session bytea,
    created_at       timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT sign_in_challenge_step_up_names_its_action CHECK ((session_id IS NULL) = (action IS NULL))
);
CREATE INDEX sign_in_challenge_by_account ON sign_in_challenge (account_id);

-- A six-digit code sent to the account's address, for one purpose, stored
-- as its SHA-256 (D5, D6). Rows are kept an hour after they are sent, which
-- is what the per-account sending limits count.
CREATE TABLE email_code (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id     uuid NOT NULL REFERENCES account (id) ON DELETE CASCADE,
    marketplace_id uuid REFERENCES marketplace (id) ON DELETE RESTRICT,
    purpose        text        NOT NULL,
    code_hash      bytea       NOT NULL,
    expires_at     timestamptz NOT NULL,
    attempts       integer     NOT NULL DEFAULT 0,
    used_at        timestamptz,
    created_at     timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT email_code_purpose_is_known CHECK (purpose IN ('signin', 'stepup', 'enrol'))
);
CREATE INDEX email_code_by_account ON email_code (account_id, created_at);

-- When the session last proved a second factor: at the sign-in that opened
-- it, or at a step-up. A sensitive action wants it less than ten minutes old
-- (D4).
ALTER TABLE session ADD COLUMN stepped_up_at timestamptz;

-- When the session last answered a code sent to the account's address that
-- is not one of its second factors, at a step-up whose action accepts one
-- (adding a card, changing the address, §18.2). It proves the address, not a
-- second factor: it never stands in for stepped_up_at before changing the
-- password or the factors (D4).
ALTER TABLE session ADD COLUMN email_confirmed_at timestamptz;

ALTER TABLE sign_in_challenge ENABLE ROW LEVEL SECURITY;
ALTER TABLE email_code ENABLE ROW LEVEL SECURITY;

CREATE POLICY sign_in_challenge_belongs_to_the_marketplace ON sign_in_challenge
    FOR ALL USING (marketplace_id = current_marketplace_id())
    WITH CHECK (marketplace_id = current_marketplace_id());
CREATE POLICY email_code_belongs_to_the_marketplace ON email_code
    FOR ALL USING (marketplace_id = current_marketplace_id())
    WITH CHECK (marketplace_id = current_marketplace_id());

-- +goose Down
ALTER TABLE session DROP COLUMN IF EXISTS email_confirmed_at;
ALTER TABLE session DROP COLUMN IF EXISTS stepped_up_at;
DROP TABLE IF EXISTS email_code;
DROP TABLE IF EXISTS sign_in_challenge;
