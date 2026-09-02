-- name: CreateCustomerPassword :execresult
INSERT INTO customer_password (customer_id, password_hash)
VALUES (?, ?);

-- name: GetCustomerPassword :one
SELECT customer_id, password_hash, created_at, updated_at
FROM customer_password
WHERE customer_id = ?;

-- name: UpdateCustomerPassword :execresult
UPDATE customer_password
SET password_hash = ?
WHERE customer_id = ?;

-- name: DeleteCustomerPassword :execresult
DELETE FROM customer_password
WHERE customer_id = ?;
