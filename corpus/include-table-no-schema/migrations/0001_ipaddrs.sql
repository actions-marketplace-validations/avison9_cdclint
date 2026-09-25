-- Reduced from Stack Overflow question 74103659 ("Debezium PostgreSQL
-- connector not creating topic", 6.9k views) and the Debezium Google Group
-- thread Kp_9jbTTEkY: the table lives in a schema, the include list names it
-- without one, the connector reports RUNNING and nothing is ever captured.
CREATE SCHEMA myschema;

CREATE TABLE myschema.ipaddrs (
    id      BIGSERIAL PRIMARY KEY,
    address INET NOT NULL,
    seen_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
