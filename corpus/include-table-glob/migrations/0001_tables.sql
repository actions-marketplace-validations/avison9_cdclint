-- Reduced from Stack Overflow question 51345636 (5.9k views): the include
-- list was written as a shell glob, public.bg_*, meaning every table whose
-- name starts with bg_. Debezium reads it as a regular expression, where *
-- repeats the character before it, so it names public.bg, public.bg_,
-- public.bg__ and no real table.
CREATE TABLE bg_orders (
    id         BIGSERIAL PRIMARY KEY,
    amount     NUMERIC(12, 2) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE bg_items (
    id       BIGSERIAL PRIMARY KEY,
    order_id BIGINT NOT NULL REFERENCES bg_orders (id),
    sku      TEXT NOT NULL
);

CREATE TABLE cp_users (
    id    BIGSERIAL PRIMARY KEY,
    email TEXT NOT NULL
);
