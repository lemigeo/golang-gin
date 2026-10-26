CREATE TABLE customer_coupon (
    id             BIGINT UNSIGNED  NOT NULL AUTO_INCREMENT,
    customer_id    BIGINT UNSIGNED  NOT NULL,
    coupon_id      BIGINT UNSIGNED  NOT NULL,
    discount_type  TINYINT UNSIGNED NOT NULL
        COMMENT '발급 시점의 coupon.discount_type 사본. 1: 금액, 2: 비율',
    discount_value INT UNSIGNED     NOT NULL
        COMMENT '발급 시점의 coupon.discount_value 사본. 원본 쿠폰이 바뀌어도 이 값으로 계산한다',
    code           VARCHAR(256) CHARACTER SET ascii COLLATE ascii_bin NULL
        COMMENT '개별 발급 쿠폰번호. 유일하지 않다',
    used_at        DATETIME         NULL COMMENT '사용 시각. NULL이면 미사용',
    expired_at     DATETIME         NULL
        COMMENT '발급 시각 + coupon.valid_period(분). NULL이면 원본 쿠폰의 ended_at을 따른다',
    created_at     DATETIME         NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '발급 시각',
    updated_at     DATETIME         NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    KEY idx_customer_coupon_customer (customer_id, used_at, expired_at),
    KEY idx_customer_coupon_customer_code (customer_id, code),
    KEY idx_customer_coupon_coupon (coupon_id),
    CONSTRAINT chk_customer_coupon_discount_value
        CHECK (discount_type <> 2 OR discount_value BETWEEN 1 AND 100)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci;
