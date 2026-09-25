-- +goose Up
-- +goose StatementBegin
ALTER TABLE orders ADD COLUMN currency CHAR(3) NOT NULL DEFAULT 'EUR';
-- +goose StatementEnd

-- +goose Down
ALTER TABLE orders DROP COLUMN currency;
