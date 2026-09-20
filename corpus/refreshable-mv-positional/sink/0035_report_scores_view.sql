-- analytics/schema/0035_report_scores_zone_columns.sql, the refreshable
-- view, with one edit a reviewer would wave through: the catchment pair
-- listed before the ward pair, beside the other area columns. Positions 9
-- to 12 now swap UUID for UUID and String for String. A refresh is an
-- INSERT ... SELECT, which is positional; between columns of the same type
-- ClickHouse writes the wrong values and reports the refresh as finished.
DROP VIEW IF EXISTS mv_report_scores;

CREATE MATERIALIZED VIEW mv_report_scores
REFRESH EVERY 1 MINUTE
TO report_scores
AS
SELECT
    r.id              AS report_id,
    r.site_id         AS site_id,
    s.scored_at       AS scored_at,
    r.created_at      AS report_created_at,
    s.dirtiness       AS dirtiness,
    s.health_risk     AS health_risk,
    s.scoring_bundle_version AS scoring_bundle_version,
    if(empty(b.maturity), 'provisional', b.maturity) AS maturity,
    c.catchment_area_id AS catchment_area_id,
    c.catchment_name    AS catchment_name,
    c.ward_area_id      AS ward_area_id,
    c.ward_name         AS ward_name,
    c.state_area_id     AS state_area_id,
    c.state_name        AS state_name,
    c.country_area_id   AS country_area_id,
    c.country_name      AS country_name,
    c.boundary_version  AS boundary_version,
    c.zone_area_id      AS zone_area_id,
    c.zone_name         AS zone_name,
    if(isNull(r.reporter_trust_score), 1.0,
       greatest(0.1, least(1.0, r.reporter_trust_score / 70.0))) AS reporter_weight
FROM (
    SELECT id, site_id, created_at, catchment_area_id, reporter_trust_score
    FROM stg_reports FINAL
    WHERE is_deleted = 0
) AS r
INNER JOIN scores AS s ON s.report_id = r.id
LEFT JOIN area_currency AS c ON c.catchment_area_id = r.catchment_area_id
LEFT JOIN stg_scoring_bundle AS b ON b.version = s.scoring_bundle_version;
