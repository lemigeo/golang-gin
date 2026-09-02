-- name: CreateCustomerSocial :execresult
INSERT INTO customer_social (customer_id, media_name, media_id)
VALUES (?, ?, ?);

-- name: GetCustomerSocial :one
SELECT id, customer_id, media_name, media_id, created_at, updated_at
FROM customer_social
WHERE id = ?;

-- name: GetCustomerSocialByMedia :one
SELECT id, customer_id, media_name, media_id, created_at, updated_at
FROM customer_social
WHERE media_name = ? AND media_id = ?;

-- name: ListCustomerSocialsByCustomer :many
SELECT id, customer_id, media_name, media_id, created_at, updated_at
FROM customer_social
WHERE customer_id = ?
ORDER BY id;

-- name: UpdateCustomerSocialMediaID :execresult
UPDATE customer_social
SET media_id = ?
WHERE id = ?;

-- name: DeleteCustomerSocial :execresult
DELETE FROM customer_social
WHERE id = ?;

-- name: DeleteCustomerSocialsByCustomer :execresult
DELETE FROM customer_social
WHERE customer_id = ?;
