-- db/migrations/0141_movement_doubt_cleared.sql (#963): a doubted capture
-- can be cleared, and the clearing is recorded on the report. The same
-- promotion carried #962, which grew the report_validations include list;
-- the first release of the diff rule saw "connector changed" and said
-- nothing about these three.
ALTER TABLE reports
    ADD COLUMN IF NOT EXISTS movement_cleared_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS movement_cleared_by UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS movement_clear_reason TEXT;
