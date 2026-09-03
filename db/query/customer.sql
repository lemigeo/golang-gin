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

-- name: GetCustomerByMedia :one
SELECT c.id, c.email, c.name, c.status, c.created_at, c.updated_at
FROM customer c
JOIN customer_social s ON s.customer_id = c.id
WHERE s.media_name = ? AND s.media_id = ?;

-- name: GetCustomerWithPasswordByEmail :one
SELECT c.id, c.email, c.name, c.status, c.created_at, c.updated_at, p.password_hash
FROM customer c
JOIN customer_password p ON p.customer_id = c.id
WHERE c.email = ?;

-- name: UpdateCustomerProfile :execresult
UPDATE customer
SET name = ?, email = ?
WHERE id = ?;
