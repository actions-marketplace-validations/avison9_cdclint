-- A guard: unwrapping first leaves a flat row, so a flatten after it changes
-- no field name and the plain column names are right.
CREATE TABLE orders (
    id    BIGSERIAL PRIMARY KEY,
    total NUMERIC(12, 2) NOT NULL
);
