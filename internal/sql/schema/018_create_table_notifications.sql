-- +goose Up
CREATE TYPE notification_status AS ENUM (
    'delivered',  -- Notification has been sent to the user
    'read',       -- Notification has been read by the user
    'pending',     -- Notification is waiting to be delivered (user is offline)
    'expired'     -- Notification has expired
);

CREATE TABLE notifications (
    id BIGSERIAL PRIMARY KEY,                     -- Unique identifier for each notification
    user_id BIGSERIAL NOT NULL REFERENCES users(id) ON DELETE CASCADE,                     -- ID of the user the notification is intended for
    message TEXT NOT NULL,                     -- The content of the notification
    notification_type VARCHAR(50) NOT NULL,    -- Type of notification (e.g., 'goal', 'market')
    status notification_status NOT NULL DEFAULT 'pending', -- Status of the notification
    created_at TIMESTAMP(0) WITH TIME ZONE NOT NULL DEFAULT NOW(), -- Timestamp when the notification was created
    updated_at TIMESTAMP(0) WITH TIME ZONE NOT NULL DEFAULT NOW(), -- Timestamp when the notification was last updated
    read_at TIMESTAMP(0) WITH TIME ZONE NULL,                    -- Timestamp when the notification was read (NULL if not read)
    expires_at TIMESTAMP(0) WITH TIME ZONE NULL,                 -- Expiration timestamp for the notification (NULL if no expiry)
    meta JSONB NULL,                           -- Extra metadata related to the notification
    redis_key VARCHAR(255) NULL               -- Redis key if stored in Redis for later delivery
);

-- Indexes for optimization
CREATE INDEX idx_notifications_user_id ON notifications (user_id);
CREATE INDEX idx_notifications_status ON notifications (status);
CREATE INDEX idx_notifications_expires_at ON notifications (expires_at);

-- +goose StatementBegin
CREATE TRIGGER trigger_update_goals_tracking_timestamp
BEFORE UPDATE ON notifications
FOR EACH ROW
EXECUTE FUNCTION update_timestamp();
-- +goose StatementEnd


-- +goose Down
DROP INDEX IF EXISTS idx_notifications_user_id;
DROP INDEX IF EXISTS idx_notifications_status;
DROP INDEX IF EXISTS idx_notifications_expires_at;
DROP TRIGGER IF EXISTS trigger_update_goals_tracking_timestamp ON notifications;
DROP TABLE IF EXISTS notifications;
DROP TYPE IF EXISTS notification_status;