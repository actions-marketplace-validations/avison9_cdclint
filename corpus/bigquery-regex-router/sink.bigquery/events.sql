CREATE TABLE `acme-analytics.cdc.events` (
    id               STRING NOT NULL,
    kind             STRING,
    payload          JSON,
    occurred_at      TIMESTAMP,
    _kafka_partition INT64
)
PARTITION BY DATE(occurred_at)
OPTIONS (description = "Fed by the BigQuery sink connector from topic events");
