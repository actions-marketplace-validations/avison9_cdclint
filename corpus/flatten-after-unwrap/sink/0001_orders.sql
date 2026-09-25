CREATE TABLE shop.orders
(
    `id` Int64,
    `total` String
)
ENGINE = MergeTree
ORDER BY id;
