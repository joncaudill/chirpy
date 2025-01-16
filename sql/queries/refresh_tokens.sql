-- name: CreateRefreshToken :one
INSERT INTO refresh_tokens (token, created_at, updated_at, user_id, expires_at, revoked_at)
VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6
)
RETURNING *;

-- name: GetRefreshToken :one
SELECT token FROM refresh_tokens
WHERE token = $1 AND expires_at > NOW() AND revoked_at IS NULL;

-- name: GetUserFromRefreshToken :one
SELECT user_id FROM refresh_tokens
WHERE token = $1;

-- name: RevokeRefreshToken :exec
UPDATE refresh_tokens
SET revoked_at = NOW(), updated_at = NOW()
WHERE token = $1;

-- name: ResetTokens :exec
DELETE FROM refresh_tokens;