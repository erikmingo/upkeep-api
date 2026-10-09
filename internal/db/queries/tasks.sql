-- name: CreateTask :one
INSERT INTO tasks (home_id, rule_id, title, detail, interval_days, season_start, season_end, start_date)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetTask :one
SELECT * FROM tasks WHERE id = $1;

-- name: ListActiveTasks :many
SELECT * FROM tasks WHERE home_id = $1 AND archived_at IS NULL ORDER BY id;

-- name: ArchiveTask :exec
UPDATE tasks SET archived_at = now() WHERE id = $1 AND archived_at IS NULL;

-- name: CreateCompletion :one
INSERT INTO completions (task_id, member_id, completed_at) VALUES ($1, $2, $3) RETURNING *;

-- name: LatestCompletions :many
SELECT DISTINCT ON (c.task_id) c.*
FROM completions c JOIN tasks t ON t.id = c.task_id
WHERE t.home_id = $1
ORDER BY c.task_id, c.completed_at DESC;
