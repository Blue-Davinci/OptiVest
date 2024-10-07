-- name: CreateNewNotification :one
INSERT INTO notifications (
    user_id, 
    message, 
    notification_type, 
    status, 
    expires_at,
    meta, 
    redis_key
)VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, created_at, updated_at;

-- name: UpdateNotificationReadAtAndStatus :one
UPDATE notifications
SET 
    status = $1,
    read_at = $2
WHERE id = $3
RETURNING updated_at;

-- name: GetUnreadNotifications :many
SELECT
    id,
    user_id,
    message,
    notification_type,
    status,
    created_at,
    updated_at,
    read_at,
    expires_at,
    meta,
    redis_key
FROM notifications
WHERE user_id = $1 AND status = 'pending' AND expires_at > NOW();