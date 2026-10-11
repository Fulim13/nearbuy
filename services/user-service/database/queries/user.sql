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

-- name: Follow :execrows
-- ON CONFLICT DO NOTHING so a double-tap is a no-op rather than an error. The
-- original has no unique constraint at all, so following twice inflates the
-- target's follower count permanently.
INSERT INTO follow (user_id, follow_user_id)
VALUES ($1, $2)
ON CONFLICT (user_id, follow_user_id) DO NOTHING;

-- name: Unfollow :execrows
DELETE FROM follow
WHERE user_id = $1 AND follow_user_id = $2;

-- name: IsFollowing :one
SELECT EXISTS (
  SELECT 1 FROM follow WHERE user_id = $1 AND follow_user_id = $2
);

-- name: ListFollowing :many
SELECT follow_user_id FROM follow WHERE user_id = $1 ORDER BY follow_user_id;

-- name: CommonFollowing :many
-- The SQL equivalent of Redis SINTER, kept as the fallback when the Redis mirror is cold.
SELECT f1.follow_user_id
FROM follow f1
JOIN follow f2 ON f2.follow_user_id = f1.follow_user_id
WHERE f1.user_id = sqlc.arg('a')
  AND f2.user_id = sqlc.arg('b')
ORDER BY f1.follow_user_id;


-- name: CountFollowing :one
SELECT count(*) FROM follow WHERE user_id = $1;

-- name: CountFollowers :one
SELECT count(*) FROM follow WHERE follow_user_id = $1;
