-- db/migrations/0033_report_validations.sql, reduced to the columns the
-- warehouse reads plus assigned_by_user_id, which it never did.
CREATE TABLE report_validations (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    report_id           UUID NOT NULL REFERENCES reports(id),
    validator_user_id   UUID NOT NULL REFERENCES users(id),
    assigned_by_user_id UUID REFERENCES users(id),
    clearance_event_id  UUID REFERENCES clearance_events(id),
    response            TEXT NOT NULL DEFAULT 'pending',
    assigned_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    responded_at        TIMESTAMPTZ
);
