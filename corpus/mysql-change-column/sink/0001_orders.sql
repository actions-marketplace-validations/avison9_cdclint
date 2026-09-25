CREATE TABLE kafka_orders
(
    `id` UInt64,
    `total_amount` String
)
ENGINE = Kafka
SETTINGS
    kafka_broker_list = '${KAFKA_BROKERS}',
    kafka_topic_list = 'shopdb.shop.orders',
    kafka_group_name = 'clickhouse-orders',
    kafka_format = 'JSONEachRow';
