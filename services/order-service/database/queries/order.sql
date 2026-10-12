-- name: CreateOrder :execrows
-- Saga S1's forward action. ON CONFLICT DO NOTHING so a redelivery that somehow
-- gets past the barrier is still harmless -- defence in depth, since the barrier
-- should already have skipped it.
INSERT INTO voucher_order (id, user_id, voucher_id, shop_id, pay_value, status)
VALUES ($1, $2, $3, $4, $5, 'unpaid')
ON CONFLICT (id) DO NOTHING;

-- name: DeleteOrder :execrows
-- Saga S1's compensation. A DELETE rather than status='cancelled': if S1 is
-- being unwound, the order never legitimately existed -- there was never enough
-- stock for it. A cancelled row would show the user an order they never had.
DELETE FROM voucher_order WHERE id = $1;

-- name: GetOrder :one
SELECT * FROM voucher_order WHERE id = $1;

-- name: ListOrdersByUser :many
SELECT * FROM voucher_order
WHERE user_id = $1
ORDER BY id DESC
LIMIT $2 OFFSET $3;

-- name: ListExpiredUnpaid :many
-- Drives the S3 expiry sweep. The WHERE clause matches lesson 02's
-- idx_order_status_create exactly -- (status, create_time) -- so this stays an
-- index scan as voucher_order grows.
--
-- LIMIT, always: an unbounded sweep after an outage would submit tens of
-- thousands of Sagas in one tick and DTM would be the thing that fell over.
SELECT id, user_id, voucher_id
FROM voucher_order
WHERE status = 'unpaid'
  AND create_time < now() - sqlc.arg('older_than')::interval
ORDER BY create_time
LIMIT sqlc.arg('max_rows');
