-- +goose Up
CREATE TABLE users (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  name VARCHAR(100) NOT NULL,
  email VARCHAR(150) NOT NULL UNIQUE,
  password_hash VARCHAR(100) NOT NULL,
  role ENUM('owner','manajer','buyer','kasir') NOT NULL,
  branch_id BIGINT NULL,
  active TINYINT(1) NOT NULL DEFAULT 1,
  created_at DATETIME NOT NULL,
  CONSTRAINT fk_users_branch FOREIGN KEY (branch_id) REFERENCES branches(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE sessions (
  token CHAR(64) PRIMARY KEY,
  user_id BIGINT NOT NULL,
  expires_at DATETIME NOT NULL,
  CONSTRAINT fk_sessions_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE audit_logs (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  user_name VARCHAR(100) NOT NULL,
  action VARCHAR(60) NOT NULL,
  entity VARCHAR(40) NOT NULL,
  entity_id BIGINT NULL,
  old_value VARCHAR(300) NULL,
  new_value VARCHAR(300) NULL,
  created_at DATETIME NOT NULL,
  KEY idx_audit_time (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE cash_sessions (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  branch_id BIGINT NOT NULL,
  opened_by VARCHAR(100) NOT NULL,
  opening_balance BIGINT NOT NULL,
  opened_at DATETIME NOT NULL,
  closed_at DATETIME NULL,
  expected_balance BIGINT NULL,
  counted_balance BIGINT NULL,
  difference BIGINT NULL,
  status ENUM('buka','menunggu','tutup') NOT NULL DEFAULT 'buka',
  CONSTRAINT fk_cash_branch FOREIGN KEY (branch_id) REFERENCES branches(id),
  KEY idx_cash_branch (branch_id, status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE cash_movements (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  session_id BIGINT NULL,
  branch_id BIGINT NOT NULL,
  kind ENUM('sale','purchase','expense','in','out') NOT NULL,
  method ENUM('tunai','transfer','qris') NOT NULL,
  amount BIGINT NOT NULL,
  note VARCHAR(200) NULL,
  created_by VARCHAR(100) NOT NULL,
  created_at DATETIME NOT NULL,
  CONSTRAINT fk_moves_session FOREIGN KEY (session_id) REFERENCES cash_sessions(id),
  CONSTRAINT fk_moves_branch FOREIGN KEY (branch_id) REFERENCES branches(id),
  KEY idx_moves_branch (branch_id, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- +goose Down
DROP TABLE IF EXISTS cash_movements;
DROP TABLE IF EXISTS cash_sessions;
DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS users;
