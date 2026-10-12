-- name: CreateOrder :execrows
-- Saga S1's forward action. ON CONFLICT DO NOTHING so a redelivery that somehow
-- gets past the barrier is still harmless -- defence in depth, since the barrier
-- should already have skipped it.
INSERT INTO voucher_order (id, user_id, voucher_id, pay_value, status)
VALUES ($1, $2, $3, $4, 'unpaid')
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
