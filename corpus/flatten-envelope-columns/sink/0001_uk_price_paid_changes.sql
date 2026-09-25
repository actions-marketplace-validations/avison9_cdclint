CREATE TABLE default.uk_price_paid_changes
(
    `before.id` Nullable(UInt64),
    `before.price` Nullable(UInt32),
    `before.postcode1` Nullable(String),
    `before.postcode2` Nullable(String),
    `before.town` Nullable(String),
    `after.id` Nullable(UInt64),
    `after.price` Nullable(UInt32),
    `after.postcode1` Nullable(String),
    `after.postcode2` Nullable(String),
    `after.town` Nullable(String),
    `op` LowCardinality(String),
    `ts_ms` UInt64
)
ENGINE = MergeTree
ORDER BY tuple();
