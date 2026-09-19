-- The counter behind the limit on sensitive endpoints.
--
-- Cloud Run runs several instances at once, so a counter held in one process
-- gives an attacker the limit once per instance. This table is where the
-- instances agree (docs/roadmap.md, F7).
--
-- It carries no `marketplace_id` and is deliberately outside row-level
-- security: what it holds is an attempt against the platform, counted before
-- anyone is identified and often before a marketplace is even resolved. A
-- policy keyed on a marketplace would refuse to count exactly the requests
-- that most need counting.

-- +goose Up

CREATE TABLE rate_limit (
    -- What is being limited, as the application spells it: the endpoint and
    -- the client, joined. Opaque here on purpose — this table counts, it does
    -- not interpret.
    subject      text        NOT NULL,
    -- The start of the window this row counts, truncated by the application so
    -- that every instance writes the same row without coordinating.
    window_start timestamptz NOT NULL,
    hits         integer     NOT NULL DEFAULT 0,

    PRIMARY KEY (subject, window_start)
);

-- For the sweep that forgets windows that have passed.
CREATE INDEX rate_limit_by_window ON rate_limit (window_start);

-- +goose Down

DROP TABLE rate_limit;
