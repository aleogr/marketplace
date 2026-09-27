-- The identity tables' rows belong to a marketplace or, for staff, to the
-- platform (docs/superpowers/specs/2026-09-26-f15-console-design.md, D4).
--
-- Until now every identity policy was `marketplace_id = current_marketplace_id()`,
-- which a row with no marketplace never satisfies: a staff account existed in
-- the schema (`account.kind = 'staff'`, migration 00009) and could never be
-- read or written by the application role. The policies become
-- `IS NOT DISTINCT FROM`, the form the outbox, the mail log and the audit log
-- already use (migrations 00006, 00007, 00008): a transaction that names a
-- marketplace still sees only that marketplace's rows, and one that names none
-- sees the platform's — staff's — and nothing else. Neither sees the other's;
-- internal/identity/platform_integration_test.go proves both directions for
-- every table below.
--
-- **The sweep's index.** A B-tree index answers `=` and `IS NULL` but not
-- `IS NOT DISTINCT FROM`, so the policy alone no longer narrows the hourly
-- DELETE of stale sessions to one scope through `session_by_marketplace`
-- (migration 00012). The sweep therefore names its scope itself
-- (internal/identity.sweepSessions), which the index does answer; the policy
-- is still what guarantees the scope.
--
-- **The lookup by e-mail's index.** The same is true of `account_email_is_unique`
-- (migration 00009), which leads with `marketplace_id`: the policy is no
-- longer an index condition, so a query that names only `email_normalised`
-- (internal/identity.accountByEmail) can no longer use that index to find its
-- handful of rows, and every sign-in reads every account instead. A second
-- index, on `email_normalised` alone, is what the lookup uses; the policy
-- still filters what it finds to the calling scope.

-- +goose Up
CREATE INDEX account_by_email ON account (email_normalised);

DROP POLICY account_belongs_to_the_marketplace ON account;
CREATE POLICY account_belongs_to_its_scope ON account
    FOR ALL USING (marketplace_id IS NOT DISTINCT FROM current_marketplace_id())
    WITH CHECK (marketplace_id IS NOT DISTINCT FROM current_marketplace_id());

DROP POLICY credential_belongs_to_the_marketplace ON credential;
CREATE POLICY credential_belongs_to_its_scope ON credential
    FOR ALL USING (marketplace_id IS NOT DISTINCT FROM current_marketplace_id())
    WITH CHECK (marketplace_id IS NOT DISTINCT FROM current_marketplace_id());

DROP POLICY email_verification_belongs_to_the_marketplace ON email_verification;
CREATE POLICY email_verification_belongs_to_its_scope ON email_verification
    FOR ALL USING (marketplace_id IS NOT DISTINCT FROM current_marketplace_id())
    WITH CHECK (marketplace_id IS NOT DISTINCT FROM current_marketplace_id());

DROP POLICY session_belongs_to_the_marketplace ON session;
CREATE POLICY session_belongs_to_its_scope ON session
    FOR ALL USING (marketplace_id IS NOT DISTINCT FROM current_marketplace_id())
    WITH CHECK (marketplace_id IS NOT DISTINCT FROM current_marketplace_id());

DROP POLICY second_factor_belongs_to_the_marketplace ON second_factor;
CREATE POLICY second_factor_belongs_to_its_scope ON second_factor
    FOR ALL USING (marketplace_id IS NOT DISTINCT FROM current_marketplace_id())
    WITH CHECK (marketplace_id IS NOT DISTINCT FROM current_marketplace_id());

DROP POLICY recovery_code_belongs_to_the_marketplace ON recovery_code;
CREATE POLICY recovery_code_belongs_to_its_scope ON recovery_code
    FOR ALL USING (marketplace_id IS NOT DISTINCT FROM current_marketplace_id())
    WITH CHECK (marketplace_id IS NOT DISTINCT FROM current_marketplace_id());

DROP POLICY factor_enrolment_belongs_to_the_marketplace ON factor_enrolment;
CREATE POLICY factor_enrolment_belongs_to_its_scope ON factor_enrolment
    FOR ALL USING (marketplace_id IS NOT DISTINCT FROM current_marketplace_id())
    WITH CHECK (marketplace_id IS NOT DISTINCT FROM current_marketplace_id());

DROP POLICY sign_in_challenge_belongs_to_the_marketplace ON sign_in_challenge;
CREATE POLICY sign_in_challenge_belongs_to_its_scope ON sign_in_challenge
    FOR ALL USING (marketplace_id IS NOT DISTINCT FROM current_marketplace_id())
    WITH CHECK (marketplace_id IS NOT DISTINCT FROM current_marketplace_id());

DROP POLICY email_code_belongs_to_the_marketplace ON email_code;
CREATE POLICY email_code_belongs_to_its_scope ON email_code
    FOR ALL USING (marketplace_id IS NOT DISTINCT FROM current_marketplace_id())
    WITH CHECK (marketplace_id IS NOT DISTINCT FROM current_marketplace_id());

DROP POLICY second_factor_failure_belongs_to_the_marketplace ON second_factor_failure;
CREATE POLICY second_factor_failure_belongs_to_its_scope ON second_factor_failure
    FOR ALL USING (marketplace_id IS NOT DISTINCT FROM current_marketplace_id())
    WITH CHECK (marketplace_id IS NOT DISTINCT FROM current_marketplace_id());

-- +goose Down
DROP INDEX account_by_email;

DROP POLICY IF EXISTS account_belongs_to_its_scope ON account;
CREATE POLICY account_belongs_to_the_marketplace ON account
    FOR ALL USING (marketplace_id = current_marketplace_id())
    WITH CHECK (marketplace_id = current_marketplace_id());

DROP POLICY IF EXISTS credential_belongs_to_its_scope ON credential;
CREATE POLICY credential_belongs_to_the_marketplace ON credential
    FOR ALL USING (marketplace_id = current_marketplace_id())
    WITH CHECK (marketplace_id = current_marketplace_id());

DROP POLICY IF EXISTS email_verification_belongs_to_its_scope ON email_verification;
CREATE POLICY email_verification_belongs_to_the_marketplace ON email_verification
    FOR ALL USING (marketplace_id = current_marketplace_id())
    WITH CHECK (marketplace_id = current_marketplace_id());

DROP POLICY IF EXISTS session_belongs_to_its_scope ON session;
CREATE POLICY session_belongs_to_the_marketplace ON session
    FOR ALL USING (marketplace_id = current_marketplace_id())
    WITH CHECK (marketplace_id = current_marketplace_id());

DROP POLICY IF EXISTS second_factor_belongs_to_its_scope ON second_factor;
CREATE POLICY second_factor_belongs_to_the_marketplace ON second_factor
    FOR ALL USING (marketplace_id = current_marketplace_id())
    WITH CHECK (marketplace_id = current_marketplace_id());

DROP POLICY IF EXISTS recovery_code_belongs_to_its_scope ON recovery_code;
CREATE POLICY recovery_code_belongs_to_the_marketplace ON recovery_code
    FOR ALL USING (marketplace_id = current_marketplace_id())
    WITH CHECK (marketplace_id = current_marketplace_id());

DROP POLICY IF EXISTS factor_enrolment_belongs_to_its_scope ON factor_enrolment;
CREATE POLICY factor_enrolment_belongs_to_the_marketplace ON factor_enrolment
    FOR ALL USING (marketplace_id = current_marketplace_id())
    WITH CHECK (marketplace_id = current_marketplace_id());

DROP POLICY IF EXISTS sign_in_challenge_belongs_to_its_scope ON sign_in_challenge;
CREATE POLICY sign_in_challenge_belongs_to_the_marketplace ON sign_in_challenge
    FOR ALL USING (marketplace_id = current_marketplace_id())
    WITH CHECK (marketplace_id = current_marketplace_id());

DROP POLICY IF EXISTS email_code_belongs_to_its_scope ON email_code;
CREATE POLICY email_code_belongs_to_the_marketplace ON email_code
    FOR ALL USING (marketplace_id = current_marketplace_id())
    WITH CHECK (marketplace_id = current_marketplace_id());

DROP POLICY IF EXISTS second_factor_failure_belongs_to_its_scope ON second_factor_failure;
CREATE POLICY second_factor_failure_belongs_to_the_marketplace ON second_factor_failure
    FOR ALL USING (marketplace_id = current_marketplace_id())
    WITH CHECK (marketplace_id = current_marketplace_id());
