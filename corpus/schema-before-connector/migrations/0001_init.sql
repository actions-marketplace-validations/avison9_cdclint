-- db/migrations/0001_init.sql and the migrations up to 0075, reduced to
-- the reports table as it stood the day 0076 added reference_number.
CREATE TABLE IF NOT EXISTS reports (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    category             TEXT NOT NULL,
    description          TEXT,
    validation_status    TEXT NOT NULL DEFAULT 'pending',
    deletion_state       TEXT NOT NULL DEFAULT 'active',
    catchment_area_id    UUID,
    site_id              UUID,
    reporter_trust_score DOUBLE PRECISION,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);
ALTER TABLE reports REPLICA IDENTITY FULL;
