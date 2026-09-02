-- name: CreateCustomer :execresult
INSERT INTO customer (email, name, status)
VALUES (?, ?, ?);

-- name: GetCustomer :one
SELECT id, email, name, status, created_at, updated_at
FROM customer
WHERE id = ?;

-- name: GetCustomerByEmail :one
SELECT id, email, name, status, created_at, updated_at
FROM customer
WHERE email = ?;

-- name: ListCustomersByStatus :many
SELECT id, email, name, status, created_at, updated_at
FROM customer
WHERE status = ?
ORDER BY id DESC
LIMIT ? OFFSET ?;

-- name: CountCustomersByStatus :one
SELECT COUNT(*)
FROM customer
WHERE status = ?;

-- name: UpdateCustomerName :execresult
UPDATE customer
SET name = ?
WHERE id = ?;

-- name: UpdateCustomerStatus :execresult
UPDATE customer
SET status = ?
WHERE id = ?;

-- name: DeleteCustomer :execresult
DELETE FROM customer
WHERE id = ?;
