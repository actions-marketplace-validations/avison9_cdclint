-- db/migrations/0035_validation_response_distance.sql: how far from the
-- report the validator stood when answering. Added, indexed, written by the
-- service on every response, and never put on the connector's include
-- list. A year later a console screen asked for it.
ALTER TABLE report_validations ADD COLUMN response_distance_m DOUBLE PRECISION;
