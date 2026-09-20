-- Spark SQL form. The lake table was created from the source table's full
-- column list; the connector excludes the phone number on purpose.
CREATE TABLE lake.cdc.shipments (
    id           string,
    status       string,
    driver_phone string,
    updated_at   timestamp
)
USING iceberg
PARTITIONED BY (days(updated_at));
