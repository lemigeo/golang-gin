-- name: CreateCoupon :execresult
INSERT INTO coupon (
    name, description, thumbnail_url, type, discount_type, discount_value,
    issue_limit_per_person, issue_limit, code, min_price, valid_period,
    status, started_at, ended_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetCoupon :one
SELECT id, name, description, thumbnail_url, type, discount_type, discount_value,
       issue_limit_per_person, issue_limit, code, min_price, valid_period,
       status, started_at, ended_at, created_at, updated_at
FROM coupon
WHERE id = ?;

-- name: ListCouponsByCode :many
-- 같은 코드를 쓰는 모든 쿠폰. 코드가 유일하지 않으므로 운영 화면에서
-- 중복 현황을 확인할 때 쓴다.
SELECT id, name, description, thumbnail_url, type, discount_type, discount_value,
       issue_limit_per_person, issue_limit, code, min_price, valid_period,
       status, started_at, ended_at, created_at, updated_at
FROM coupon
WHERE code = ?
ORDER BY id DESC;

-- name: GetCouponByCode :one
-- 코드는 유일하지 않으므로 id와 함께 지정한다. 두 조건이 같이 걸리면
-- 대상이 하나로 정해지므로 LIMIT 없이 한 행이 나온다.
SELECT id, name, description, thumbnail_url, type, discount_type, discount_value,
       issue_limit_per_person, issue_limit, code, min_price, valid_period,
       status, started_at, ended_at, created_at, updated_at
FROM coupon
WHERE id = ? AND code = ?;

-- name: ListCoupons :many
SELECT id, name, description, thumbnail_url, type, discount_type, discount_value,
       issue_limit_per_person, issue_limit, code, min_price, valid_period,
       status, started_at, ended_at, created_at, updated_at
FROM coupon
ORDER BY id DESC
LIMIT ? OFFSET ?;

-- name: CountCoupons :one
SELECT COUNT(*) FROM coupon;

-- name: ListDownloadableCoupons :many
SELECT id, name, description, thumbnail_url, type, discount_type, discount_value,
       issue_limit_per_person, issue_limit, code, min_price, valid_period,
       status, started_at, ended_at, created_at, updated_at
FROM coupon
WHERE status = 1
  AND type = 1
  AND code IS NULL
  AND (started_at IS NULL OR started_at <= sqlc.arg(now))
  AND (ended_at IS NULL OR ended_at >= sqlc.arg(now))
ORDER BY id DESC
LIMIT ? OFFSET ?;

-- name: UpdateCoupon :execresult
-- PUT의 의미대로 수정 가능한 모든 컬럼을 덮어쓴다. 생략된 필드는 이전 값이
-- 남는 게 아니라 NULL이 된다.
UPDATE coupon
SET name = ?, description = ?, thumbnail_url = ?, type = ?,
    discount_type = ?, discount_value = ?,
    issue_limit_per_person = ?, issue_limit = ?, code = ?, min_price = ?,
    valid_period = ?, status = ?, started_at = ?, ended_at = ?
WHERE id = ?;

-- name: UpdateCouponStatus :execresult
UPDATE coupon
SET status = ?
WHERE id = ?;

-- name: DeleteCoupon :execresult
DELETE FROM coupon
WHERE id = ?;
