-- Ensure the uuid_generate_v4() function is available.
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TYPE property_type AS ENUM ('rent', 'sale', 'shortlet');

CREATE TABLE listings (
    id          UUID             PRIMARY KEY DEFAULT uuid_generate_v4(),
    title       VARCHAR(255)     NOT NULL,
    -- NUMERIC preserves exact decimal precision for currency values.
    price       NUMERIC(15, 2)   NOT NULL CHECK (price >= 0),
    type        property_type    NOT NULL,
    bedrooms    INTEGER          NOT NULL CHECK (bedrooms >= 0),
    -- Stored as plain DOUBLE PRECISION columns rather than a PostGIS geometry.
    -- The Haversine formula is applied in SQL at query time using these values.
    latitude    DOUBLE PRECISION NOT NULL CHECK (latitude  >= -90  AND latitude  <= 90),
    longitude   DOUBLE PRECISION NOT NULL CHECK (longitude >= -180 AND longitude <= 180),
    agent_id    UUID             NOT NULL,
    created_at  TIMESTAMPTZ      NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ      NOT NULL DEFAULT NOW()
);

-- Trigger to keep updated_at current on every row update.
CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER listings_set_updated_at
    BEFORE UPDATE ON listings
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

-- Indexes chosen to support the common filter combinations in the List query:
--   type filter (exact match)
--   price range (inequality scan)
--   bedrooms filter (exact match)
--   agent_id lookup
--   default sort order (created_at DESC)
--
-- Composite indexes are omitted deliberately — the data set is small and the
-- query planner will intersect single-column indexes efficiently. Composite
-- indexes become worthwhile once query patterns are established from production
-- traffic.
CREATE INDEX idx_listings_type       ON listings (type);
CREATE INDEX idx_listings_price      ON listings (price);
CREATE INDEX idx_listings_bedrooms   ON listings (bedrooms);
CREATE INDEX idx_listings_agent_id   ON listings (agent_id);
CREATE INDEX idx_listings_created_at ON listings (created_at DESC);
