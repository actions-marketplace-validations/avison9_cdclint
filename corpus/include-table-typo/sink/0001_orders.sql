CREATE TABLE kafka_orders
(
    `id` Int64,
    `customer_id` Int64,
    `total` String
)
ENGINE = Kafka
SETTINGS
    kafka_broker_list = '${KAFKA_BROKERS}',
    kafka_topic_list = 'shop.public.orders',
    kafka_group_name = 'clickhouse-orders',
    kafka_format = 'JSONEachRow';
