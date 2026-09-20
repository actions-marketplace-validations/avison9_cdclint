-- analytics/schema/0001_report_scores.sql, 0006_zone_tier.sql and 0007_reporter_trust.sql:
-- the target table and the two migrations that appended columns to it.
CREATE TABLE IF NOT EXISTS report_scores
(
    report_id           UUID,
    site_id             UUID,

    scored_at           DateTime64(3, 'UTC'),
    report_created_at   DateTime64(3, 'UTC'),

    dirtiness           UInt8,
    health_risk         UInt8,

    scoring_bundle_version String,
    maturity            LowCardinality(String),

    ward_area_id        UUID,
    ward_name           String,
    catchment_area_id   UUID,
    catchment_name      String,
    state_area_id       UUID,
    state_name          String,
    country_area_id     UUID,
    country_name        String,

    boundary_version    String
)
ENGINE = ReplacingMergeTree(scored_at)
ORDER BY (country_area_id, catchment_area_id, report_id)
;
ALTER TABLE report_scores ADD COLUMN IF NOT EXISTS zone_area_id UUID;
ALTER TABLE report_scores ADD COLUMN IF NOT EXISTS zone_name    String;
ALTER TABLE report_scores ADD COLUMN IF NOT EXISTS reporter_weight Float64;
