-- db/migrations/0076_report_reference_number.sql, reduced to the statement that shapes the table.
ALTER TABLE reports ADD COLUMN IF NOT EXISTS reference_number BIGINT;
