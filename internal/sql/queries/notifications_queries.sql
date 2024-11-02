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
WHERE user_id = $1 
AND status = 'pending' 
AND (expires_at > NOW() OR expires_at IS NULL);

-- name: GetAllExpiredNotifications :many
SELECT
    COUNT(*) OVER() AS total_count,
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
WHERE expires_at < NOW()
AND status = 'pending'
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: GetAllNotificationsByUserId :many
SELECT
    COUNT(*) OVER() AS total_count,
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
WHERE user_id = $1
AND ($2 = '' OR to_tsvector('simple', notification_type) @@ plainto_tsquery('simple', $2))
ORDER BY created_at DESC
LIMIT $3 OFFSET $4;