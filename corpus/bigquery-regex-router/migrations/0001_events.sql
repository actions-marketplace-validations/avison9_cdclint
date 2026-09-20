CREATE TABLE events (
    id          UUID PRIMARY KEY,
    kind        TEXT NOT NULL,
    payload     JSONB,
    occurred_at TIMESTAMPTZ NOT NULL
);
