-- analytics/schema/0024_report_reference_number.sql: the Kafka-engine table recreated with the new field.
DROP TABLE IF EXISTS kafka_reports;
CREATE TABLE kafka_reports
(
    `id`                   String,
    `category`             String,
    `description`          String,
    `validation_status`    String,
    `deletion_state`       String,
    `catchment_area_id`    String,
    `site_id`              String,
    `created_at`           String,
    `updated_at`           String,
    `reporter_trust_score` Nullable(Float64),
    -- Nullable end to end, and not because the source column is: 0076
    -- makes it NOT NULL. It is Nullable HERE because a row that was
    -- streamed before the connector carried this column arrives without
    -- it, and toInt64OrZero on an absent field would silently store 0 --
    -- a reference that renders RPT-00000 and names no report. NULL says
    -- "not delivered", which is what the backfill check below reads.
    --
    -- Int64, NOT UInt64: the source column is BIGINT, which is signed, and
    -- the renderer takes an int64. Carrying it unsigned bought nothing --
    -- the sequence starts at 1 and never goes near either bound -- while
    -- costing a cast at the only place it is read.
    `reference_number`     Nullable(Int64),
    `__op`                 String,
    `__deleted`            String,
    `__lsn`                UInt64,
    `__ts_us`              UInt64
)
ENGINE = Kafka
SETTINGS
    kafka_broker_list = '${KAFKA_BROKERS}',
    kafka_topic_list = 'rr.public.reports',
    kafka_group_name = 'clickhouse-rollup-reports',
    kafka_format = 'JSONEachRow',
    input_format_skip_unknown_fields = 1,
    kafka_skip_broken_messages = 100;
