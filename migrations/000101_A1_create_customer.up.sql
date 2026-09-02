CREATE TABLE customer (
    id         BIGINT UNSIGNED  NOT NULL AUTO_INCREMENT,
    email      VARCHAR(255)     NOT NULL,
    name       VARCHAR(64)      NOT NULL,
    status     TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '0: deleted, 1: active, 2: inactive',
    created_at DATETIME         NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME         NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY uk_customer_email (email),
    KEY idx_customer_status_id (status, id)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci;
