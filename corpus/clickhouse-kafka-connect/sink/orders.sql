-- Written by the ClickHouse Kafka Connect sink, not by a Kafka-engine table,
-- so the topic comes from the sink connector's topic2TableMap.
CREATE TABLE shop.orders
(
    id          UUID,
    customer_id UUID,
    total_cents Int64,
    currency    FixedString(3),
    created_at  DateTime64(6, 'UTC')
)
ENGINE = ReplacingMergeTree
ORDER BY id;
