-- +goose Up
CREATE TABLE branches (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  name VARCHAR(100) NOT NULL UNIQUE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE categories (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  name VARCHAR(100) NOT NULL UNIQUE,
  target_margin_bp INT NOT NULL DEFAULT 1200,
  sort_order INT NOT NULL DEFAULT 0
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE brands (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  category_id BIGINT NOT NULL,
  name VARCHAR(100) NOT NULL,
  CONSTRAINT fk_brands_category FOREIGN KEY (category_id) REFERENCES categories(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE product_models (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  brand_id BIGINT NOT NULL,
  name VARCHAR(150) NOT NULL,
  prefix VARCHAR(8) NOT NULL,
  CONSTRAINT fk_models_brand FOREIGN KEY (brand_id) REFERENCES brands(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE variants (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  model_id BIGINT NOT NULL,
  name VARCHAR(100) NOT NULL,
  CONSTRAINT fk_variants_model FOREIGN KEY (model_id) REFERENCES product_models(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE grades (
  code CHAR(1) PRIMARY KEY,
  label VARCHAR(50) NOT NULL,
  factor_bp INT NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE accessories (
  code VARCHAR(30) PRIMARY KEY,
  label VARCHAR(50) NOT NULL,
  deduction BIGINT NOT NULL DEFAULT 0
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE price_observations (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  model_id BIGINT NOT NULL,
  variant_id BIGINT NOT NULL,
  grade CHAR(1) NOT NULL,
  series ENUM('sell','buy') NOT NULL,
  source ENUM('toko','pesaing','marketplace','penjual') NOT NULL,
  price BIGINT NOT NULL,
  accessories JSON NULL,
  observed_at DATETIME NOT NULL,
  is_outlier TINYINT(1) NOT NULL DEFAULT 0,
  note VARCHAR(200) NULL,
  CONSTRAINT fk_obs_model FOREIGN KEY (model_id) REFERENCES product_models(id),
  CONSTRAINT fk_obs_variant FOREIGN KEY (variant_id) REFERENCES variants(id),
  KEY idx_obs_lookup (model_id, variant_id, grade, series, observed_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE purchases (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  branch_id BIGINT NOT NULL,
  model_id BIGINT NOT NULL,
  variant_id BIGINT NOT NULL,
  grade CHAR(1) NOT NULL,
  seller_name VARCHAR(100) NOT NULL,
  seller_phone VARCHAR(30) NULL,
  deal_price BIGINT NOT NULL,
  est_cost BIGINT NOT NULL DEFAULT 0,
  status ENUM('menunggu','disetujui','ditolak') NOT NULL DEFAULT 'disetujui',
  created_at DATETIME NOT NULL,
  CONSTRAINT fk_purchase_branch FOREIGN KEY (branch_id) REFERENCES branches(id),
  CONSTRAINT fk_purchase_model FOREIGN KEY (model_id) REFERENCES product_models(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE units (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  code VARCHAR(30) NOT NULL UNIQUE,
  purchase_id BIGINT NULL,
  branch_id BIGINT NOT NULL,
  model_id BIGINT NOT NULL,
  variant_id BIGINT NOT NULL,
  grade CHAR(1) NOT NULL,
  accessories JSON NULL,
  serial VARCHAR(80) NULL,
  cost BIGINT NOT NULL,
  listed_price BIGINT NOT NULL,
  min_price BIGINT NOT NULL,
  status ENUM('dicek','servis','siap','menunggu','terjual','retur') NOT NULL,
  created_at DATETIME NOT NULL,
  sold_at DATETIME NULL,
  CONSTRAINT fk_units_purchase FOREIGN KEY (purchase_id) REFERENCES purchases(id),
  CONSTRAINT fk_units_branch FOREIGN KEY (branch_id) REFERENCES branches(id),
  CONSTRAINT fk_units_model FOREIGN KEY (model_id) REFERENCES product_models(id),
  KEY idx_units_status (status, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE sales (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  invoice VARCHAR(30) NOT NULL UNIQUE,
  unit_id BIGINT NOT NULL UNIQUE,
  branch_id BIGINT NOT NULL,
  price BIGINT NOT NULL,
  discount BIGINT NOT NULL DEFAULT 0,
  method ENUM('tunai','transfer','qris') NOT NULL,
  buyer_name VARCHAR(100) NULL,
  warranty_days INT NOT NULL DEFAULT 0,
  profit BIGINT NOT NULL,
  sold_at DATETIME NOT NULL,
  CONSTRAINT fk_sales_unit FOREIGN KEY (unit_id) REFERENCES units(id),
  CONSTRAINT fk_sales_branch FOREIGN KEY (branch_id) REFERENCES branches(id),
  KEY idx_sales_date (sold_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE approvals (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  kind ENUM('purchase','price','cash') NOT NULL,
  ref_id BIGINT NULL,
  branch_id BIGINT NULL,
  title VARCHAR(200) NOT NULL,
  requester VARCHAR(100) NOT NULL,
  reason VARCHAR(300) NULL,
  amount BIGINT NOT NULL DEFAULT 0,
  compare_amount BIGINT NOT NULL DEFAULT 0,
  status ENUM('menunggu','disetujui','ditolak') NOT NULL DEFAULT 'menunggu',
  created_at DATETIME NOT NULL,
  decided_at DATETIME NULL,
  decided_by VARCHAR(100) NULL,
  KEY idx_approvals_status (status, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE settings (
  k VARCHAR(60) PRIMARY KEY,
  v VARCHAR(200) NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- +goose Down
DROP TABLE IF EXISTS settings;
DROP TABLE IF EXISTS approvals;
DROP TABLE IF EXISTS sales;
DROP TABLE IF EXISTS units;
DROP TABLE IF EXISTS purchases;
DROP TABLE IF EXISTS price_observations;
DROP TABLE IF EXISTS accessories;
DROP TABLE IF EXISTS grades;
DROP TABLE IF EXISTS variants;
DROP TABLE IF EXISTS product_models;
DROP TABLE IF EXISTS brands;
DROP TABLE IF EXISTS categories;
DROP TABLE IF EXISTS branches;
