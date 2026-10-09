-- name: UpsertRule :one
INSERT INTO rules (id, version, definition) VALUES ($1, $2, $3)
ON CONFLICT (id) DO UPDATE SET version = EXCLUDED.version, definition = EXCLUDED.definition, updated_at = now()
RETURNING *;

-- name: ListRules :many
SELECT * FROM rules ORDER BY id;
