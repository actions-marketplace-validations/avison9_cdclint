-- The warehouse side, waiting on the topic Debezium would produce for
-- myschema.ipaddrs once the table is captured.
CREATE TABLE kafka_ipaddrs
(
    `id` Int64,
    `address` String,
    `seen_at` String
)
ENGINE = Kafka
SETTINGS
    kafka_broker_list = '${KAFKA_BROKERS}',
    kafka_topic_list = 'test.myschema.ipaddrs',
    kafka_group_name = 'clickhouse-ipaddrs',
    kafka_format = 'JSONEachRow';
