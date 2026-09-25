-- goose keeps both halves of a migration in one file. The down section
-- comes after the up one, so a reader that applies the whole file creates
-- the table and drops it again.
-- +goose Up
CREATE TABLE orders (
    id    BIGSERIAL PRIMARY KEY,
    total NUMERIC(12, 2) NOT NULL
);

-- +goose Down
DROP TABLE orders;
