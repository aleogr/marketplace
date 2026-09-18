-- Markets and marketplaces.
--
-- A market groups what a country decides: its currency, the payment methods it
-- allows, its address format, and the tax and consumer rules that apply there.
-- Brazil is a row in `market`, never a branch in the code
-- (docs/requirements.md, section 5).
--
-- Rules are data **with effective dates**, because they change and the change
-- has a date: Brazilian import taxes changed in May 2026 while competitors'
-- help pages still showed the old ones. An order placed before a change must
-- still be explicable by the rules that applied to it, which a table that only
-- holds today's answer cannot do.
--
-- No table here carries `marketplace_id`: these describe the platform and its
-- marketplaces, not data belonging to one. Row-level security arrives with the
-- tenant tables it protects (docs/design.md, section 2.6).

-- +goose Up

CREATE TABLE market (
    code           text PRIMARY KEY,
    name           text        NOT NULL,
    -- ISO 4217. Money is stored in minor units with its currency, always
    -- (docs/requirements.md, section 6).
    currency       char(3)     NOT NULL,
    -- The named shape of an address in this market. A format, not a country
    -- branch: addresses must assume no country's layout.
    address_format text        NOT NULL,
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT market_code_is_a_country CHECK (code ~ '^[A-Z]{2}$'),
    CONSTRAINT market_currency_is_iso_4217 CHECK (currency ~ '^[A-Z]{3}$')
);

CREATE TRIGGER market_updated_at
    BEFORE UPDATE ON market
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Which payment methods a market allows, and when.
--
-- `effective_to` is null while a method is current. The range is half-open, so
-- a method that ends on the day another begins leaves no day uncovered and no
-- day covered twice.
CREATE TABLE market_payment_method (
    market_code    text NOT NULL REFERENCES market (code) ON DELETE RESTRICT,
    method         text NOT NULL,
    effective_from date NOT NULL,
    effective_to   date,

    PRIMARY KEY (market_code, method, effective_from),
    CONSTRAINT market_payment_method_ends_after_it_starts
        CHECK (effective_to IS NULL OR effective_to > effective_from)
);

-- The tax and consumer rules of a market, as data.
--
-- The value is JSON because the shape differs by kind and this delivery does
-- not know every kind yet; what it does know is that a rule has a kind, a
-- value, and the dates between which it applied.
CREATE TABLE market_rule (
    id             bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    market_code    text        NOT NULL REFERENCES market (code) ON DELETE RESTRICT,
    kind           text        NOT NULL,
    value          jsonb       NOT NULL,
    effective_from date        NOT NULL,
    effective_to   date,
    created_at     timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT market_rule_ends_after_it_starts
        CHECK (effective_to IS NULL OR effective_to > effective_from)
);

CREATE INDEX market_rule_by_market_and_kind
    ON market_rule (market_code, kind, effective_from DESC);

CREATE TABLE marketplace (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    -- A stable, human-readable handle. The display name can change; this
    -- cannot, because it ends up in operational places.
    slug          text        NOT NULL UNIQUE,
    name          text        NOT NULL,
    market_code   text        NOT NULL REFERENCES market (code) ON DELETE RESTRICT,

    -- Commission on each sale, or paid listings. The vehicles marketplace
    -- launches on listings and charges no commission; the electronics one is
    -- the opposite (docs/design.md, decision 2).
    revenue_model text        NOT NULL,

    -- A marketplace exists before it answers. It leaves `in_preparation` only
    -- when its host answers and its e-mail domain verifies (docs/design.md,
    -- decision 16), which is why resolution refuses a host whose marketplace
    -- is still being prepared.
    state         text        NOT NULL DEFAULT 'in_preparation',

    -- The per-marketplace flags of design decision 2. Masking contact details
    -- is porous and the vehicles revenue does not depend on it, so detection
    -- is on for goods and off for vehicles, and a store may show its telephone.
    detect_contact_data boolean NOT NULL DEFAULT true,
    reveal_contact      boolean NOT NULL DEFAULT false,

    -- BCP 47, canonical case. The default is what a visitor gets when the
    -- address carries no language and nothing is remembered
    -- (docs/requirements.md, section 6).
    default_language text NOT NULL DEFAULT 'en-US',

    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT marketplace_slug_is_a_handle CHECK (slug ~ '^[a-z0-9]([a-z0-9-]*[a-z0-9])?$'),
    CONSTRAINT marketplace_revenue_model_is_known
        CHECK (revenue_model IN ('commission', 'paid_listings')),
    CONSTRAINT marketplace_state_is_known
        CHECK (state IN ('in_preparation', 'active'))
);

CREATE TRIGGER marketplace_updated_at
    BEFORE UPDATE ON marketplace
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- The languages a marketplace serves. en-US and pt-BR from the start, and a
-- new one is a row and a translation file, never a code change
-- (docs/requirements.md, section 6).
CREATE TABLE marketplace_language (
    marketplace_id uuid NOT NULL REFERENCES marketplace (id) ON DELETE CASCADE,
    language       text NOT NULL,

    PRIMARY KEY (marketplace_id, language)
);

-- The hosts a marketplace answers on.
--
-- The host of the request decides which marketplace serves it, from day one
-- (docs/requirements.md, section 7). Cloud Run's domain mapping supports no
-- wildcard, so each host is also an infrastructure step; the table is what the
-- application reads, and it is deliberately not the same list.
--
-- Stored lower-case with no port: a host is case-insensitive and the port is
-- not part of the name. Resolution normalises before it looks, so that
-- `MARKETPLACE1.example:8080` and `marketplace1.example` are one host.
CREATE TABLE marketplace_host (
    host           text PRIMARY KEY,
    marketplace_id uuid        NOT NULL REFERENCES marketplace (id) ON DELETE RESTRICT,
    created_at     timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT marketplace_host_is_normalised CHECK (host = lower(host)),
    CONSTRAINT marketplace_host_carries_no_port CHECK (host !~ ':')
);

CREATE INDEX marketplace_host_by_marketplace ON marketplace_host (marketplace_id);

-- +goose Down

DROP TABLE marketplace_host;
DROP TABLE marketplace_language;
DROP TABLE marketplace;
DROP TABLE market_rule;
DROP TABLE market_payment_method;
DROP TABLE market;
