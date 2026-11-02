-- 초기 스키마. 티켓 단위가 아니라 프로젝트 베이스라인이므로 파일명에 티켓
-- ID를 붙이지 않는다. 이후 변경은 티켓 단위 마이그레이션으로 추가한다.
--
-- 테이블 생성 순서는 참조 방향을 따른다: customer 계열 → coupon → customer_coupon.
-- 외래키는 걸지 않지만, 순서를 맞춰두면 나중에 FK를 추가하기로 바뀌어도
-- 이 파일을 다시 쓸 필요가 없다.

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

CREATE TABLE customer_social (
    id          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    customer_id BIGINT UNSIGNED NOT NULL,
    media_name  VARCHAR(64)     NOT NULL,
    media_id    VARCHAR(255)    NOT NULL,
    created_at  DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY uk_customer_social_media (media_name, media_id),
    UNIQUE KEY uk_customer_social_customer_media (customer_id, media_name),
    KEY idx_customer_social_customer_id (customer_id)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci;

CREATE TABLE customer_password (
    customer_id   BIGINT UNSIGNED NOT NULL,
    password_hash VARCHAR(255) CHARACTER SET ascii COLLATE ascii_bin NOT NULL
        COMMENT 'Argon2id PHC 인코딩 문자열. 알고리즘과 파라미터가 값 안에 포함된다.',
    created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (customer_id)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci;

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
