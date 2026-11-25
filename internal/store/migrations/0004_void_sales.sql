-- +goose Up
ALTER TABLE sales
  ADD COLUMN voided_at DATETIME NULL,
  ADD COLUMN voided_by VARCHAR(100) NULL,
  ADD COLUMN void_reason VARCHAR(200) NULL;

CREATE INDEX idx_sales_voided ON sales (voided_at);

-- +goose Down
ALTER TABLE sales DROP COLUMN void_reason, DROP COLUMN voided_by, DROP COLUMN voided_at;
