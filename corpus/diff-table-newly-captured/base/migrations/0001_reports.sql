-- db/migrations/0001_initial.sql, reduced to the reports columns the
-- connector captures.
CREATE TABLE reports (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    reporter_user_id  UUID NOT NULL REFERENCES users(id),
    category          TEXT NOT NULL,
    description       TEXT,
    validation_status TEXT NOT NULL DEFAULT 'pending',
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);
