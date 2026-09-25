-- Reduced from ClickHouse's own Postgres CDC example (ClickHouse/examples,
-- cdc/postgresql at ae417ae, and the blog post "ClickHouse PostgreSQL change
-- data capture, part 2"): the change events keep Debezium's envelope, a
-- flatten transform names the fields before.<column> and after.<column>, and
-- the ClickHouse table names its columns the same way.
CREATE TABLE uk_price_paid (
    id        BIGSERIAL PRIMARY KEY,
    price     INTEGER NOT NULL,
    postcode1 TEXT,
    postcode2 TEXT,
    town      TEXT
);
