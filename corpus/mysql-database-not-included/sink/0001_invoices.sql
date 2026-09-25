CREATE TABLE kafka_invoices
(
    `id` UInt64,
    `order_id` UInt64,
    `issued_at` String
)
ENGINE = Kafka
SETTINGS
    kafka_broker_list = '${KAFKA_BROKERS}',
    kafka_topic_list = 'shopdb.billing.invoices',
    kafka_group_name = 'clickhouse-invoices',
    kafka_format = 'JSONEachRow';
