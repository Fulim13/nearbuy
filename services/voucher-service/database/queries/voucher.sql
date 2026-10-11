-- name: CreateVoucher :one
INSERT INTO voucher (shop_id, title, sub_title, rules, pay_value, actual_value, type)
VALUES ($1, $2, $3, $4, $5, $6, 'normal')
RETURNING *;

-- name: CreateSeckillVoucherRow :one
-- The voucher half of a seckill voucher. Paired with InsertSeckillStock inside
-- one transaction -- a voucher of type 'seckill' with no seckill_voucher row is
-- a broken state the API must never expose.
INSERT INTO voucher (shop_id, title, sub_title, rules, pay_value, actual_value, type)
VALUES ($1, $2, $3, $4, $5, $6, 'seckill')
RETURNING *;

-- name: InsertSeckillStock :exec
INSERT INTO seckill_voucher (voucher_id, stock, begin_time, end_time)
VALUES ($1, $2, $3, $4);

-- name: GetVoucher :one
-- LEFT JOIN so a normal voucher comes back with NULL seckill columns rather than
-- no row at all. sqlc turns those into pointers, which is how the service tells
-- the two kinds apart without a second query.
SELECT v.*, sv.stock, sv.begin_time, sv.end_time
FROM voucher v
LEFT JOIN seckill_voucher sv ON sv.voucher_id = v.id
WHERE v.id = $1;

-- name: ListVouchersByShop :many
SELECT v.*, sv.stock, sv.begin_time, sv.end_time
FROM voucher v
LEFT JOIN seckill_voucher sv ON sv.voucher_id = v.id
WHERE v.shop_id = $1 AND v.status = 'active'
ORDER BY v.id;

-- name: ListSeckillVouchers :many
-- Used at startup to warm Redis with the stock of every live seckill.
SELECT sv.voucher_id, sv.stock
FROM seckill_voucher sv
JOIN voucher v ON v.id = sv.voucher_id
WHERE v.status = 'active' AND sv.end_time > now();

-- name: DeductStock :execrows
-- The forward action of Saga S1
--
-- There is no "AND stock > 0" guard here on purpose: CHECK (stock >= 0)
-- and a constraint violation is an ERROR the Saga can act on. A WHERE guard would instead return 0 rows affected,
-- which is indistinguishable from "that voucher does not exist".
UPDATE seckill_voucher
SET stock = stock - sqlc.arg('qty')::int
WHERE voucher_id = sqlc.arg('voucher_id');

-- name: RestoreStock :execrows
-- The compensation for DeductStock.
UPDATE seckill_voucher
SET stock = stock + sqlc.arg('qty')::int
WHERE voucher_id = sqlc.arg('voucher_id');

-- name: GetSeckillStock :one
SELECT stock, begin_time, end_time FROM seckill_voucher WHERE voucher_id = $1;
