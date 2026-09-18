-- The trigger function every table with an `updated_at` column will use.
--
-- It is here rather than in the delivery that adds the first such table
-- because it belongs to none of them: repeating it in each migration that
-- needs it is how two versions of the same function end up in one schema.
--
-- No domain table is created in this delivery (docs/roadmap.md, F4).

-- +goose Up
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION set_updated_at() RETURNS trigger AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP FUNCTION IF EXISTS set_updated_at();
-- +goose StatementEnd
