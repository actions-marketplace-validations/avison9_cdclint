# The change under review adds a column to a captured table and leaves the
# connector alone.
ALTER TABLE orders ADD COLUMN coupon_code VARCHAR(32) NULL;
