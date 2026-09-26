-- Consecutive failed second-factor answers, per account
-- (docs/superpowers/specs/2026-09-25-f14-two-factor-design.md, D8; the
-- owner's decision of 2026-09-25): at ten the owner is told by e-mail, at a
-- hundred the codes an app shows or an e-mail carries are refused until the
-- password is changed.
--
-- The count has a table of its own rather than a column of account. An
-- answer locks its challenge and then the account's factor rows, while
-- removing a factor locks the account and then its factor rows: writing the
-- account after the factor rows could close a cycle between the two. Every
-- path takes this row last, after the factor rows and the audit chain, and
-- takes nothing after it.
--
-- No row is a count of zero: a right answer and a password change delete it.

-- +goose Up
CREATE TABLE second_factor_failure (
    account_id     uuid PRIMARY KEY REFERENCES account (id) ON DELETE CASCADE,
    marketplace_id uuid REFERENCES marketplace (id) ON DELETE RESTRICT,
    failures       integer     NOT NULL,
    updated_at     timestamptz NOT NULL,

    CONSTRAINT second_factor_failure_counts_one_at_least CHECK (failures > 0)
);

ALTER TABLE second_factor_failure ENABLE ROW LEVEL SECURITY;

CREATE POLICY second_factor_failure_belongs_to_the_marketplace ON second_factor_failure
    FOR ALL USING (marketplace_id = current_marketplace_id())
    WITH CHECK (marketplace_id = current_marketplace_id());

-- +goose Down
DROP TABLE IF EXISTS second_factor_failure;
