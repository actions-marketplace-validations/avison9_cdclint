-- A second database on the same server, created by the same migrations.
-- The connector's database.include.list names only shop, so nothing from
-- billing is captured, whatever the table lists say.
USE billing;
CREATE TABLE invoices (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  order_id BIGINT UNSIGNED NOT NULL,
  issued_at DATETIME NOT NULL
);
