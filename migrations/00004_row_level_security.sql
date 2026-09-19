-- Row-level security: the database refuses what a forgotten filter would let
-- through.
--
-- Isolation is enforced twice (docs/design.md, section 2.6). The application
-- scopes every query by the marketplace of the request; this is the second
-- belt, and it is the one that still holds when a query forgets. A connection
-- that has not said which marketplace it is serving sees nothing at all — not
-- "everything", which is what an unscoped query returns without these policies.
--
-- **Who is subject to this.** A table's owner is not, and that is deliberate:
-- migrations and the seed run as the owning role and must see every
-- marketplace at once. The service connects as a different role, which owns
-- nothing, and is therefore subject to every policy below.
--
-- **The one way in.** Host resolution runs before any marketplace is known —
-- it is what decides which one — so it cannot be scoped by the answer it is
-- looking for. Rather than leave the routing tables unprotected, it reads them
-- through `tenancy_host_map()`, which runs as the owner and returns nothing
-- but the routing map. One named, auditable exception beats three tables the
-- policies do not cover.

-- +goose Up

-- +goose StatementBegin
CREATE FUNCTION current_marketplace_id() RETURNS uuid
    LANGUAGE sql
    STABLE
    -- Not SECURITY DEFINER: it reads the caller's own setting.
    SET search_path = pg_catalog
AS $$
    -- `true` means "missing is not an error": a connection that has set
    -- nothing gets NULL, and NULL matches no row, which is the whole point.
    SELECT nullif(current_setting('app.marketplace_id', true), '')::uuid
$$;
-- +goose StatementEnd

COMMENT ON FUNCTION current_marketplace_id() IS
    'The marketplace the current transaction is serving, or NULL. Set with SET LOCAL by internal/platform/db.';

ALTER TABLE marketplace ENABLE ROW LEVEL SECURITY;
ALTER TABLE marketplace_language ENABLE ROW LEVEL SECURITY;
ALTER TABLE marketplace_host ENABLE ROW LEVEL SECURITY;

-- FOR ALL, with the same expression as USING and as WITH CHECK: a row of
-- another marketplace can be neither read nor written, and a row cannot be
-- written into a marketplace the transaction is not serving either.
CREATE POLICY marketplace_is_the_one_being_served ON marketplace
    FOR ALL
    USING (id = current_marketplace_id())
    WITH CHECK (id = current_marketplace_id());

CREATE POLICY marketplace_language_belongs_to_the_marketplace ON marketplace_language
    FOR ALL
    USING (marketplace_id = current_marketplace_id())
    WITH CHECK (marketplace_id = current_marketplace_id());

CREATE POLICY marketplace_host_belongs_to_the_marketplace ON marketplace_host
    FOR ALL
    USING (marketplace_id = current_marketplace_id())
    WITH CHECK (marketplace_id = current_marketplace_id());

-- The routing map, and nothing else.
--
-- SECURITY DEFINER, so it runs as the owner and the policies above do not
-- apply to it. `search_path` is fixed, because a SECURITY DEFINER function
-- that resolves names through the caller's search path is how a caller
-- substitutes its own table for the one the author meant.
-- +goose StatementBegin
CREATE FUNCTION tenancy_host_map()
    RETURNS TABLE (
        host                text,
        marketplace_id      text,
        slug                text,
        name                text,
        market_code         text,
        revenue_model       text,
        state               text,
        detect_contact_data boolean,
        reveal_contact      boolean,
        default_language    text,
        languages           text[]
    )
    LANGUAGE sql
    STABLE
    SECURITY DEFINER
    SET search_path = public, pg_temp
AS $$
    SELECT h.host,
           m.id::text,
           m.slug,
           m.name,
           m.market_code,
           m.revenue_model,
           m.state,
           m.detect_contact_data,
           m.reveal_contact,
           m.default_language,
           COALESCE(
               ARRAY(
                   SELECT l.language
                   FROM marketplace_language l
                   WHERE l.marketplace_id = m.id
                   ORDER BY l.language
               ),
               ARRAY[]::text[]
           )
    FROM marketplace_host h
    JOIN marketplace m ON m.id = h.marketplace_id
$$;
-- +goose StatementEnd

COMMENT ON FUNCTION tenancy_host_map() IS
    'Every host and the marketplace behind it, for resolution, which runs before a marketplace is known.';

-- Nobody by default: the grant is made to the application role by name, where
-- every other privilege of that role is granted (internal/platform/db).
REVOKE ALL ON FUNCTION tenancy_host_map() FROM PUBLIC;

-- +goose Down

DROP FUNCTION IF EXISTS tenancy_host_map();

DROP POLICY IF EXISTS marketplace_host_belongs_to_the_marketplace ON marketplace_host;
DROP POLICY IF EXISTS marketplace_language_belongs_to_the_marketplace ON marketplace_language;
DROP POLICY IF EXISTS marketplace_is_the_one_being_served ON marketplace;

ALTER TABLE marketplace_host DISABLE ROW LEVEL SECURITY;
ALTER TABLE marketplace_language DISABLE ROW LEVEL SECURITY;
ALTER TABLE marketplace DISABLE ROW LEVEL SECURITY;

DROP FUNCTION IF EXISTS current_marketplace_id();
