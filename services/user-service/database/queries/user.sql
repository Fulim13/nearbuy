-- name: GetUser :one
SELECT * FROM app_user WHERE id = $1;

-- name: GetUserByPhone :one
SELECT * FROM app_user WHERE phone = $1;

-- name: CreateUser :one
INSERT INTO app_user (phone, nick_name)
VALUES ($1, $2)
RETURNING *;

-- name: UpdateUserProfile :execrows
-- sqlc.narg = nullable arg, so NULL means "not supplied"
-- COALESCE keeps the existing column value
UPDATE app_user
SET nick_name = COALESCE(sqlc.narg('nick_name'), nick_name),
    icon      = COALESCE(sqlc.narg('icon'), icon),
    city      = COALESCE(sqlc.narg('city'), city),
    introduce = COALESCE(sqlc.narg('introduce'), introduce)
WHERE id = sqlc.arg('id');

-- name: AdjustBalance :execrows
UPDATE app_user
SET balance = balance + sqlc.arg('amount')
WHERE id = sqlc.arg('id');

-- name: CreateUserByPhone :one
-- Get-or-create a user by phone.
--
-- Why DO UPDATE instead of DO NOTHING:
--   1. A inserts +60123 → success, gets the row
--   2. B inserts +60123 → conflict → DO NOTHING → no row returned → B's login fails
--
-- EXCLUDED.phone = the value we tried to insert (same as the existing one),
-- so the SET changes nothing, but Postgres counts the row as updated and
-- RETURNING gives it back.
INSERT INTO app_user (phone, nick_name)
VALUES ($1, $2)
ON CONFLICT (phone) DO UPDATE SET phone = EXCLUDED.phone
RETURNING *;
