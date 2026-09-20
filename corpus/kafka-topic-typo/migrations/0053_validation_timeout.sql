ALTER TABLE report_validations
    ADD COLUMN IF NOT EXISTS closed_at TIMESTAMPTZ;
