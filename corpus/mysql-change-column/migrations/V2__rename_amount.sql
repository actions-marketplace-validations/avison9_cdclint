-- MySQL renames a column with CHANGE, which also restates its type. The
-- include list still names amount, which no longer exists, and the new
-- name is on no list, so the renamed column stops arriving.
ALTER TABLE orders CHANGE COLUMN amount total_amount DECIMAL(14,2) NOT NULL;
