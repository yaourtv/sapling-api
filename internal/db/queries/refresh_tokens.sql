-- name: CreateRefreshToken :exec
INSERT INTO refresh_tokens (user_id, token_hash, expires_at)
VALUES ($1, $2, $3);

-- name: GetRefreshTokenByHash :one
SELECT * FROM refresh_tokens WHERE token_hash = $1;

-- name: DeleteRefreshTokenByHash :execrows
DELETE FROM refresh_tokens WHERE token_hash = $1;

-- name: DeleteExpiredRefreshTokens :exec
DELETE FROM refresh_tokens WHERE expires_at < now();

-- name: DeleteRefreshTokensForUser :exec
DELETE FROM refresh_tokens WHERE user_id = $1;
