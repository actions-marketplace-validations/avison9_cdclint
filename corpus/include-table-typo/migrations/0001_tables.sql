-- The plain shape: one include-list entry misspelled. Debezium logs a
-- warning when an entry matches nothing and keeps running (Debezium issue
-- dbz#872 asks it to fail instead), so the orders topic is simply never
-- produced.
CREATE TABLE customers (
    id    BIGSERIAL PRIMARY KEY,
    email TEXT NOT NULL
);

CREATE TABLE orders (
    id          BIGSERIAL PRIMARY KEY,
    customer_id BIGINT NOT NULL REFERENCES customers (id),
    total       NUMERIC(12, 2) NOT NULL
);
