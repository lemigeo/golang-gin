-- name: DownloadCoupon :execresult
-- 검증과 발급을 한 문장에 담는다. 조건이 하나라도 어긋나면 SELECT가 행을
-- 내놓지 않으므로 INSERT도 일어나지 않고, RowsAffected가 0으로 돌아온다.
-- 할인 조건은 발급 시점 값을 그대로 복사해 이후 쿠폰이 수정돼도 영향받지 않는다.
INSERT INTO customer_coupon (
    customer_id, coupon_id, discount_type, discount_value, expired_at
)
SELECT sqlc.arg(customer_id), c.id, c.discount_type, c.discount_value,
       IF(c.valid_period IS NULL, NULL,
          DATE_ADD(NOW(), INTERVAL c.valid_period MINUTE))
FROM coupon c
WHERE c.id = sqlc.arg(coupon_id)
  AND c.status = 1
  AND (c.started_at IS NULL OR c.started_at <= NOW())
  AND (c.ended_at IS NULL OR c.ended_at >= NOW())
  AND (c.issue_limit IS NULL
       OR (SELECT COUNT(cc.id)
           FROM customer_coupon cc
           WHERE cc.coupon_id = c.id) < c.issue_limit)
  AND (c.issue_limit_per_person IS NULL
       OR (SELECT COUNT(cc.id)
           FROM customer_coupon cc
           WHERE cc.coupon_id = c.id
             AND cc.customer_id = sqlc.arg(customer_id)) < c.issue_limit_per_person);

-- name: GetCustomerCoupon :one
SELECT id, customer_id, coupon_id, discount_type, discount_value, code,
       used_at, expired_at, created_at, updated_at
FROM customer_coupon
WHERE id = ?;

-- name: CountCustomerCouponsByCoupon :one
SELECT COUNT(id) FROM customer_coupon WHERE coupon_id = ?;

-- name: CountCustomerCouponsByCouponAndCustomer :one
SELECT COUNT(id) FROM customer_coupon WHERE coupon_id = ? AND customer_id = ?;
