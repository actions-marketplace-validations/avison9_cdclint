-- The table as first written: the columns named as they are in Postgres.
-- The flatten transform names every field after.<column>, and the sink
-- matches fields to columns by name, so nothing matched.
CREATE TABLE default.testradiomeas
(
    `measDate` Int32,
    `measLongitude` Float64,
    `measLatitude` Float64,
    `measNetNoCoverage` Int32,
    `measNetType` String
)
ENGINE = MergeTree
ORDER BY `measDate`;
