-- analytics/schema/0020_version_rename_report_validations.sql: the Kafka-engine
-- table as deployed. Nothing here asks for response_distance_m, which is why
-- nobody noticed it was never captured.
CREATE TABLE kafka_report_validations
(
    `id` String,
    `report_id` String,
    `validator_user_id` String,
    `clearance_event_id` String,
    `response` String,
    `assigned_at` String,
    `responded_at` Nullable(String),
    `closed_at` Nullable(String),
    `movement_implausible` Nullable(String),
    `__op` String,
    `__deleted` String,
    `__lsn` UInt64,
    `__ts_us` UInt64
)
ENGINE = Kafka
SETTINGS
    kafka_broker_list = '${KAFKA_BROKERS}',
    kafka_topic_list = 'rr.public.report_validation',
    kafka_group_name = 'clickhouse-rollup-report-validations',
    kafka_format = 'JSONEachRow',
    input_format_skip_unknown_fields = 1,
    kafka_skip_broken_messages = 100;
