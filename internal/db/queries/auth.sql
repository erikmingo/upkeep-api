-- name: CreateLoginToken :one
INSERT INTO login_tokens (email, token_hash, expires_at) VALUES ($1, $2, $3) RETURNING *;

-- name: ConsumeLoginToken :one
UPDATE login_tokens SET used_at = now()
WHERE token_hash = $1 AND used_at IS NULL AND expires_at > $2
RETURNING *;

-- name: CreateSession :one
INSERT INTO sessions (user_id, token_hash, expires_at) VALUES ($1, $2, $3) RETURNING *;

-- name: GetSessionUser :one
SELECT u.* FROM sessions s JOIN users u ON u.id = s.user_id
WHERE s.token_hash = $1 AND s.expires_at > $2;

-- name: DeleteSession :exec
DELETE FROM sessions WHERE token_hash = $1;

-- name: FirstHomeForUser :one
SELECT h.*, m.id AS member_id FROM members m JOIN homes h ON h.id = m.home_id
WHERE m.user_id = $1 ORDER BY m.id LIMIT 1;
