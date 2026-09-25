-- Second factors, recovery codes and the enrolments in progress
-- (docs/superpowers/specs/2026-09-25-f14-two-factor-design.md).
--
-- An account "has 2FA" when it has at least one second_factor. Every table is
-- a marketplace's, under row-level security like every tenant table; a staff
-- account has no marketplace, so its rows are invisible to the application
-- role until staff sign in by their own path (F15), as its account is.

-- +goose Up
CREATE TABLE second_factor (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id       uuid NOT NULL REFERENCES account (id) ON DELETE CASCADE,
    marketplace_id   uuid REFERENCES marketplace (id) ON DELETE RESTRICT,
    kind             text        NOT NULL,
    -- What the person called it, to tell two keys or two phones apart.
    label            text        NOT NULL,
    -- An authenticator app: the secret, sealed with the person's own audit
    -- key (spec, D5), and the last step a code was accepted for, so no code
    -- is accepted twice (D6).
    secret           bytea,
    totp_last_step   bigint,
    -- A security key or the device's authenticator: the credential's id, its
    -- public key (COSE), its signature counter and the flags it registered
    -- with. None of them is a secret.
    credential_id    bytea,
    public_key       bytea,
    sign_count       bigint,
    credential_flags smallint,
    created_at       timestamptz NOT NULL DEFAULT now(),
    last_used_at     timestamptz,

    CONSTRAINT second_factor_kind_is_known CHECK (kind IN ('totp', 'webauthn', 'email')),
    CONSTRAINT second_factor_app_has_a_secret CHECK ((kind = 'totp') = (secret IS NOT NULL)),
    CONSTRAINT second_factor_key_has_a_credential
        CHECK ((kind = 'webauthn') = (credential_id IS NOT NULL AND public_key IS NOT NULL
                                      AND sign_count IS NOT NULL AND credential_flags IS NOT NULL)),
    CONSTRAINT second_factor_credential_is_unique UNIQUE (credential_id)
);
CREATE INDEX second_factor_by_account ON second_factor (account_id);
-- An address receives one code at a time: one e-mail factor per account.
CREATE UNIQUE INDEX second_factor_one_email_per_account ON second_factor (account_id) WHERE kind = 'email';

CREATE TABLE recovery_code (
    account_id     uuid NOT NULL REFERENCES account (id) ON DELETE CASCADE,
    marketplace_id uuid REFERENCES marketplace (id) ON DELETE RESTRICT,
    -- SHA-256 of the code's twelve symbols. The code itself was shown once
    -- and is kept nowhere.
    code_hash      bytea       NOT NULL,
    used_at        timestamptz,
    created_at     timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (account_id, code_hash)
);

-- An app or a key being added, between the page that starts it and the
-- answer that proves it works. One per session: starting another replaces
-- it, and it ends with the session.
CREATE TABLE factor_enrolment (
    session_id       uuid PRIMARY KEY REFERENCES session (id) ON DELETE CASCADE,
    account_id       uuid NOT NULL REFERENCES account (id) ON DELETE CASCADE,
    marketplace_id   uuid REFERENCES marketplace (id) ON DELETE RESTRICT,
    kind             text        NOT NULL,
    -- The app's secret, sealed as second_factor.secret is.
    secret           bytea,
    -- The WebAuthn registration ceremony's state: its challenge, which the
    -- browser's answer must carry.
    webauthn_session bytea,
    expires_at       timestamptz NOT NULL,
    created_at       timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT factor_enrolment_kind_is_known CHECK (kind IN ('totp', 'webauthn')),
    CONSTRAINT factor_enrolment_carries_its_state
        CHECK ((kind = 'totp' AND secret IS NOT NULL) OR (kind = 'webauthn' AND webauthn_session IS NOT NULL))
);

ALTER TABLE second_factor ENABLE ROW LEVEL SECURITY;
ALTER TABLE recovery_code ENABLE ROW LEVEL SECURITY;
ALTER TABLE factor_enrolment ENABLE ROW LEVEL SECURITY;

CREATE POLICY second_factor_belongs_to_the_marketplace ON second_factor
    FOR ALL USING (marketplace_id = current_marketplace_id())
    WITH CHECK (marketplace_id = current_marketplace_id());
CREATE POLICY recovery_code_belongs_to_the_marketplace ON recovery_code
    FOR ALL USING (marketplace_id = current_marketplace_id())
    WITH CHECK (marketplace_id = current_marketplace_id());
CREATE POLICY factor_enrolment_belongs_to_the_marketplace ON factor_enrolment
    FOR ALL USING (marketplace_id = current_marketplace_id())
    WITH CHECK (marketplace_id = current_marketplace_id());

-- +goose Down
DROP TABLE IF EXISTS factor_enrolment;
DROP TABLE IF EXISTS recovery_code;
DROP TABLE IF EXISTS second_factor;
