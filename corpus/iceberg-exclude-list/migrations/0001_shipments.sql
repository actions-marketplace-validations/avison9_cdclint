CREATE TABLE shipments (
    id           UUID PRIMARY KEY,
    status       TEXT NOT NULL,
    driver_phone TEXT,
    updated_at   TIMESTAMPTZ NOT NULL
);
