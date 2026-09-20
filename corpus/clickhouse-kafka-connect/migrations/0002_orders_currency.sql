-- The order's currency, added when the shop went multi-currency. The
-- warehouse table got the column the same week; the connector did not.
ALTER TABLE orders ADD COLUMN currency CHAR(3) NOT NULL DEFAULT 'NGN';
