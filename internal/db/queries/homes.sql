-- name: CreateHome :one
INSERT INTO homes (name) VALUES ($1) RETURNING *;

-- name: GetHome :one
SELECT * FROM homes WHERE id = $1;

-- name: CreateMember :one
INSERT INTO members (home_id, user_id) VALUES ($1, $2)
ON CONFLICT (home_id, user_id) DO UPDATE SET home_id = EXCLUDED.home_id
RETURNING *;

-- name: ListMembers :many
SELECT m.*, u.email, u.display_name
FROM members m JOIN users u ON u.id = m.user_id
WHERE m.home_id = $1 ORDER BY m.id;

-- name: UpsertFact :one
INSERT INTO facts (home_id, key, value, source) VALUES ($1, $2, $3, $4)
ON CONFLICT (home_id, key) DO UPDATE SET value = EXCLUDED.value, source = EXCLUDED.source, updated_at = now()
RETURNING *;

-- name: ListFacts :many
SELECT * FROM facts WHERE home_id = $1 ORDER BY key;

-- name: DeleteFact :exec
DELETE FROM facts WHERE home_id = $1 AND key = $2;
