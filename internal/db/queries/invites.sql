-- name: CreateInvite :one
INSERT INTO invites (home_id, created_by, code_hash, expires_at) VALUES ($1, $2, $3, $4) RETURNING *;

-- name: ConsumeInvite :one
UPDATE invites SET used_at = now()
WHERE code_hash = $1 AND used_at IS NULL AND expires_at > $2
RETURNING *;
