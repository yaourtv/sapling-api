-- name: CreateStatus :one
INSERT INTO statuses (user_id, name, color, position, is_system)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListStatuses :many
SELECT * FROM statuses
WHERE user_id = $1
ORDER BY position, created_at;

-- name: GetStatus :one
SELECT * FROM statuses WHERE id = $1 AND user_id = $2;

-- name: GetLostStatus :one
SELECT * FROM statuses WHERE user_id = $1 AND is_system = true;

-- name: UpdateStatus :one
UPDATE statuses
SET name = $3, color = $4, position = $5
WHERE id = $1 AND user_id = $2
RETURNING *;

-- name: DeleteStatus :execrows
DELETE FROM statuses WHERE id = $1 AND user_id = $2;

-- name: CountApplicationsWithStatus :one
SELECT count(*) FROM applications WHERE current_status_id = $1;

-- name: ReassignApplicationsStatus :many
UPDATE applications
SET current_status_id = sqlc.arg(new_status_id), updated_at = now()
WHERE user_id = sqlc.arg(user_id) AND current_status_id = sqlc.arg(old_status_id)
RETURNING id;
