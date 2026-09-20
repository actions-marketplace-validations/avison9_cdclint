-- The source side is not what this entry is about; the view reads staging
-- tables, not the stream. One captured table keeps the run honest.
CREATE TABLE reports (
    id         UUID PRIMARY KEY,
    site_id    UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
