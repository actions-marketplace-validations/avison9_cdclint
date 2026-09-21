-- db/migrations/0141_movement_doubt_cleared.sql (#963), with the fix a
-- hurried author makes: two of the three new columns go on the include
-- list in the same change and the third is forgotten. Adding two says
-- nothing about the third.
ALTER TABLE reports
    ADD COLUMN IF NOT EXISTS movement_cleared_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS movement_cleared_by UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS movement_clear_reason TEXT;
