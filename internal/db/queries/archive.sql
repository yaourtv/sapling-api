-- name: ArchiveStaleApplications :many
UPDATE applications
SET current_status_id = (
    SELECT id FROM statuses
    WHERE user_id = applications.user_id AND is_system = true
),
updated_at = now()
WHERE updated_at < now() - (
    SELECT archive_after_days * interval '1 day'
    FROM users WHERE id = applications.user_id
)
AND (SELECT archive_after_days FROM users WHERE id = applications.user_id) > 0
AND current_status_id NOT IN (
    SELECT id FROM statuses WHERE is_system = true
)
RETURNING id;

-- name: InsertArchiveHistory :exec
INSERT INTO application_status_history (application_id, status_id, status_name)
SELECT a.id, a.current_status_id, s.name
FROM applications a
JOIN statuses s ON s.id = a.current_status_id
WHERE a.id = ANY($1::uuid[]);
