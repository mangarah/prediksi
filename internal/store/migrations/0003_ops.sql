-- +goose Up
ALTER TABLE sales ADD COLUMN sold_by VARCHAR(100) NULL AFTER buyer_name;

ALTER TABLE units MODIFY COLUMN status ENUM('dicek','servis','siap','menunggu','terjual','retur','kirim') NOT NULL;

ALTER TABLE purchases
  ADD COLUMN photo VARCHAR(255) NULL,
  ADD COLUMN serial_override_reason VARCHAR(200) NULL;

CREATE TABLE transfers (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  unit_id BIGINT NOT NULL,
  from_branch BIGINT NOT NULL,
  to_branch BIGINT NOT NULL,
  sent_by VARCHAR(100) NOT NULL,
  sent_at DATETIME NOT NULL,
  received_by VARCHAR(100) NULL,
  received_at DATETIME NULL,
  status ENUM('kirim','terima') NOT NULL DEFAULT 'kirim',
  CONSTRAINT fk_transfer_unit FOREIGN KEY (unit_id) REFERENCES units(id),
  CONSTRAINT fk_transfer_from FOREIGN KEY (from_branch) REFERENCES branches(id),
  CONSTRAINT fk_transfer_to FOREIGN KEY (to_branch) REFERENCES branches(id),
  KEY idx_transfer_status (status, to_branch)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- +goose Down
DROP TABLE IF EXISTS transfers;
ALTER TABLE purchases DROP COLUMN serial_override_reason, DROP COLUMN photo;
ALTER TABLE units MODIFY COLUMN status ENUM('dicek','servis','siap','menunggu','terjual','retur') NOT NULL;
ALTER TABLE sales DROP COLUMN sold_by;
