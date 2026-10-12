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

-- name: CancelExpiredOrders :many
-- Step 1 of the expiry job: cancel unpaid orders older than the payment window.
--
-- One statement, so it cannot race a payment: PayOrder's UPDATE also requires
-- status = 'unpaid', and whichever commits first wins the row. FOR UPDATE SKIP
-- LOCKED lets several order-service replicas run the job at once, each taking
-- different rows. The inner WHERE matches idx_order_status_create.
UPDATE voucher_order
SET status = 'cancelled'
WHERE id IN (
    SELECT id FROM voucher_order
    WHERE status = 'unpaid'
      AND create_time < now() - sqlc.arg('older_than')::interval
    ORDER BY create_time
    LIMIT sqlc.arg('max_rows')
    FOR UPDATE SKIP LOCKED
)
RETURNING id;

-- name: ListRestorePending :many
-- Step 2: cancelled orders whose stock voucher-service has not yet given back.
-- Retried every tick until MarkStockRestored; idx_order_restore_pending.
SELECT id, user_id, voucher_id
FROM voucher_order
WHERE status = 'cancelled' AND stock_restored_at IS NULL
ORDER BY create_time
LIMIT $1;

-- name: MarkStockRestored :exec
UPDATE voucher_order SET stock_restored_at = now() WHERE id = $1;
