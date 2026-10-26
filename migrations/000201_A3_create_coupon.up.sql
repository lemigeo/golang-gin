CREATE TABLE coupon (
    id                     BIGINT UNSIGNED  NOT NULL AUTO_INCREMENT,
    name                   VARCHAR(512)     NOT NULL,
    description            TEXT             NOT NULL,
    thumbnail_url          VARCHAR(2048)    NULL
        COMMENT '썸네일 이미지 URL. NULL이면 이미지 없음',
    type                   TINYINT UNSIGNED NOT NULL
        COMMENT '1: 공개(다운로드), 2: 비공개(보상)',
    discount_type          TINYINT UNSIGNED NOT NULL
        COMMENT '1: 금액, 2: 비율',
    discount_value          INT UNSIGNED     NOT NULL
        COMMENT 'discount_type=1이면 할인 금액, 2이면 할인 퍼센트',
    issue_limit_per_person INT UNSIGNED     NULL
        COMMENT '인당 발행 건수. NULL = 제한 없음',
    issue_limit            INT UNSIGNED     NULL
        COMMENT '전체 발행 제한. NULL = 제한 없음',
    code                   VARCHAR(256) CHARACTER SET ascii COLLATE ascii_bin NULL
        COMMENT '단일 코드. NULL이면 코드 없이 발행되는 쿠폰',
    min_price              INT UNSIGNED     NULL
        COMMENT '최소 사용 금액. NULL = 제한 없음',
    valid_period           INT UNSIGNED     NULL
        COMMENT '발행 후 사용 가능 기간(분). NULL이면 ended_at까지',
    status                 TINYINT UNSIGNED NOT NULL DEFAULT 1
        COMMENT '0: inactive, 1: active',
    started_at             DATETIME         NULL COMMENT '시작일. NULL이면 즉시',
    ended_at               DATETIME         NULL COMMENT '종료일. NULL이면 무기한',
    created_at             DATETIME         NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at             DATETIME         NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    -- code는 유일하지 않다. 같은 코드 문자열을 여러 쿠폰이 쓸 수 있고,
    -- 어느 쿠폰이 적용되는지는 조회 조건(상태·기간)이 결정한다.
    KEY idx_coupon_code (code, status),
    KEY idx_coupon_status_type (status, type, started_at, ended_at),
    CONSTRAINT chk_coupon_discount_value
        CHECK (discount_type <> 2 OR discount_value BETWEEN 1 AND 100),
    CONSTRAINT chk_coupon_period
        CHECK (started_at IS NULL OR ended_at IS NULL OR ended_at >= started_at)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci;
