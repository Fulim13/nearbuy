-- name: GetShop :one
SELECT * FROM shop WHERE id = $1;

-- name: ListShopTypes :many
SELECT * FROM shop_type ORDER BY sort_order, id;

-- name: ListShops :many
-- The unfiltered browse list. Same ordering as the filtered ones so paging
-- through "All" and then picking a category feels consistent.
SELECT * FROM shop
ORDER BY id
LIMIT $1 OFFSET $2;

-- name: CountShops :one
SELECT count(*) FROM shop;

-- name: ListShopsByType :many
SELECT * FROM shop
WHERE type_id = $1
ORDER BY id
LIMIT $2 OFFSET $3;

-- name: CountShopsByType :one
SELECT count(*) FROM shop WHERE type_id = $1;

-- name: SearchShopsByName :many
SELECT * FROM shop
WHERE name ILIKE '%' || sqlc.arg('name')::text || '%'
ORDER BY id
LIMIT sqlc.arg('lim') OFFSET sqlc.arg('off');

-- name: CreateShop :one
INSERT INTO shop (name, type_id, images, area, address, x, y, avg_price, open_hours)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: UpdateShop :execrows
UPDATE shop
SET name 	= COALESCE(sqlc.narg('name'), name),
    type_id 	= COALESCE(sqlc.narg('type_id'), type_id),
    images 	= COALESCE(sqlc.narg('images'), images),
    area 	= COALESCE(sqlc.narg('area'), area),
    address 	= COALESCE(sqlc.narg('address'), address),
    avg_price 	= COALESCE(sqlc.narg('avg_price'), avg_price),
    open_hours 	= COALESCE(sqlc.narg('open_hours'), open_hours)
WHERE id = sqlc.arg('id');

-- name: ClaimEvent :execrows
-- Returns 1 the first time an event_key is seen and 0 on every redelivery, so
-- the caller can skip. Must run in the SAME transaction for ClaimEvent and IncrementShopSold as the effect it guards.
INSERT INTO applied_event (event_key, topic)
VALUES ($1, $2)
ON CONFLICT (event_key) DO NOTHING;

-- name: IncrementShopSold :execrows
UPDATE shop SET sold = sold + sqlc.arg('delta')::int
WHERE id = sqlc.arg('id');
