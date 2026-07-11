-- name: CreateEvent :one
INSERT INTO calendar_events (user_id, title, description, starts_at, ends_at)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetEvent :one
SELECT * FROM calendar_events WHERE id = $1 AND user_id = $2;

-- name: ListEventsInRange :many
SELECT * FROM calendar_events
WHERE user_id = $1 AND starts_at < sqlc.arg(until) AND ends_at > sqlc.arg(since)
ORDER BY starts_at;

-- name: UpdateEvent :one
UPDATE calendar_events
SET title = $3, description = $4, starts_at = $5, ends_at = $6
WHERE id = $1 AND user_id = $2
RETURNING *;

-- name: DeleteEvent :execrows
DELETE FROM calendar_events WHERE id = $1 AND user_id = $2;

-- name: CreateAvailabilityWindow :one
INSERT INTO availability_windows (user_id, is_recurring, day_of_week, specific_date, start_time, end_time)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListAvailabilityWindows :many
SELECT * FROM availability_windows
WHERE user_id = $1
ORDER BY is_recurring DESC, day_of_week, specific_date, start_time;

-- name: DeleteAvailabilityWindow :execrows
DELETE FROM availability_windows WHERE id = $1 AND user_id = $2;

-- name: CreateInvitation :one
INSERT INTO invitations (user_id, title, description, proposed_starts, proposed_ends)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetInvitation :one
SELECT * FROM invitations WHERE id = $1 AND user_id = $2;

-- name: ListInvitations :many
SELECT * FROM invitations WHERE user_id = $1 ORDER BY proposed_starts;

-- name: ListInvitationsInRange :many
SELECT * FROM invitations
WHERE user_id = $1 AND proposed_starts < sqlc.arg(until) AND proposed_ends > sqlc.arg(since)
ORDER BY proposed_starts;

-- name: DeleteInvitation :execrows
DELETE FROM invitations WHERE id = $1 AND user_id = $2;
