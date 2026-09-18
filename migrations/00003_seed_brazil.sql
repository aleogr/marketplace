-- Brazil, as data.
--
-- The first market to operate, and deliberately nothing more than a row: the
-- design must not treat Brazil as the centre of the system or as a special case
-- embedded in code (docs/requirements.md, section 1).
--
-- The effective dates below are **the platform's**, not the country's. PIX has
-- existed since 2020 and the consumer right of withdrawal since 1990, but this
-- platform has no order that predates itself, so dating the rows from when the
-- platform can first have used them says what is true instead of implying that
-- Brazilian legal history has been modelled here. The dates exist so that a
-- rule which changes later leaves the old one explicable, which is what an
-- order placed under it will need (docs/requirements.md, section 5).

-- +goose Up

INSERT INTO market (code, name, currency, address_format)
VALUES ('BR', 'Brazil', 'BRL', 'br');

INSERT INTO market_payment_method (market_code, method, effective_from)
VALUES
    ('BR', 'pix', DATE '2026-01-01'),
    ('BR', 'credit_card', DATE '2026-01-01');

INSERT INTO market_rule (market_code, kind, value, effective_from)
VALUES (
    'BR',
    'right_of_withdrawal',
    -- Seven days from receipt, for anything bought at a distance. Whether it
    -- reaches vehicles is an open question for a lawyer, and until it is
    -- answered the vehicle reservation is always refunded, which is a decision
    -- recorded in the design rather than a number here
    -- (docs/requirements.md, section 29).
    '{"days": 7, "applies_to": "distance_sales"}'::jsonb,
    DATE '2026-01-01'
);

-- +goose Down

DELETE FROM market_rule WHERE market_code = 'BR';
DELETE FROM market_payment_method WHERE market_code = 'BR';
DELETE FROM market WHERE code = 'BR';
