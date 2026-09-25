CREATE TABLE kafka_bg_orders
(
    `id` Int64,
    `amount` String,
    `created_at` String
)
ENGINE = Kafka
SETTINGS
    kafka_broker_list = '${KAFKA_BROKERS}',
    kafka_topic_list = 'shop.public.bg_orders',
    kafka_group_name = 'clickhouse-bg-orders',
    kafka_format = 'JSONEachRow';
