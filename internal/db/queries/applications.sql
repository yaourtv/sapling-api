-- name: CreateApplication :one
INSERT INTO applications (user_id, company, position, link, description, current_status_id, applied_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetApplication :one
SELECT sqlc.embed(a), s.name AS status_name, s.color AS status_color,
  coalesce((SELECT max(h.changed_at) FROM application_status_history h
   WHERE h.application_id = a.id), a.created_at)::timestamptz AS last_status_change
FROM applications a
JOIN statuses s ON s.id = a.current_status_id
WHERE a.id = $1 AND a.user_id = $2;

-- name: ListApplications :many
SELECT sqlc.embed(a), s.name AS status_name, s.color AS status_color,
  coalesce((SELECT max(h.changed_at) FROM application_status_history h
   WHERE h.application_id = a.id), a.created_at)::timestamptz AS last_status_change
FROM applications a
JOIN statuses s ON s.id = a.current_status_id
WHERE a.user_id = $1
  AND (sqlc.arg(include_lost)::boolean = true OR s.is_system = false)
ORDER BY a.applied_at DESC, a.created_at DESC;

-- name: UpdateApplication :one
UPDATE applications
SET company = $3, position = $4, link = $5, description = $6, applied_at = $7, updated_at = now()
WHERE id = $1 AND user_id = $2
RETURNING *;

-- name: UpdateApplicationStatus :one
UPDATE applications
SET current_status_id = $3, updated_at = now()
WHERE id = $1 AND user_id = $2
RETURNING *;

-- name: DeleteApplication :execrows
DELETE FROM applications WHERE id = $1 AND user_id = $2;

-- name: InsertStatusHistory :one
INSERT INTO application_status_history (application_id, status_id, status_name)
VALUES ($1, $2, $3)
RETURNING *;

-- name: ListStatusHistory :many
SELECT h.* FROM application_status_history h
JOIN applications a ON a.id = h.application_id
WHERE h.application_id = $1 AND a.user_id = $2
ORDER BY h.changed_at DESC;
