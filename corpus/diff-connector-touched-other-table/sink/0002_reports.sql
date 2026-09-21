-- analytics/schema/0002_reports.sql: the Kafka-engine table for reports as
-- deployed. It asks for nothing #963 added, so the static rules are quiet;
-- only the diff rule can see the three new columns.
CREATE TABLE kafka_reports
(
    `id` String,
    `reporter_user_id` String,
    `category` String,
    `description` Nullable(String),
    `validation_status` String,
    `created_at` String,
    `updated_at` String,
    `__op` String,
    `__deleted` String,
    `__lsn` UInt64,
    `__ts_us` UInt64
)
ENGINE = Kafka
SETTINGS
    kafka_broker_list = '${KAFKA_BROKERS}',
    kafka_topic_list = 'rr.public.reports',
    kafka_group_name = 'clickhouse-rollup-reports',
    kafka_format = 'JSONEachRow',
    input_format_skip_unknown_fields = 1,
    kafka_skip_broken_messages = 100;
