-- name: CreateUser :one
INSERT INTO users (email, password_hash, display_name, timezone, calendar_token)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = $1;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: GetUserByCalendarToken :one
SELECT * FROM users WHERE calendar_token = $1;

-- name: UpdateUserProfile :one
UPDATE users
SET display_name = $2, timezone = $3, archive_after_days = $4, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: UpdateUserCalendarToken :one
UPDATE users
SET calendar_token = $2, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: UpdateUserPassword :exec
UPDATE users
SET password_hash = $2, updated_at = now()
WHERE id = $1;
