-- +goose Up
CREATE TABLE groups (
    id BIGSERIAL PRIMARY KEY,                                   -- Unique group ID
    creator_user_id BIGINT REFERENCES users(id) ON DELETE CASCADE, -- User who created the group
    group_image_url TEXT NOT NULL,                              -- Group image URL
    name VARCHAR(255) NOT NULL,                                 -- Group name (e.g., "Family Saving Group")
    is_private BOOLEAN DEFAULT TRUE,                            -- If the group is private or public
    max_member_count INTEGER DEFAULT 10,                        -- Maximum number of members allowed
    description TEXT,                                           -- Group description
    activity_count INTEGER DEFAULT 0,                           -- Number of activities in the recent period
    last_activity_at TIMESTAMP(0) WITH TIME ZONE DEFAULT NOW(), -- Last activity date
    created_at TIMESTAMP(0) WITH TIME ZONE DEFAULT NOW(),             -- Creation date
    updated_at TIMESTAMP(0) WITH TIME ZONE DEFAULT NOW(),              -- Last updated
    version INTEGER DEFAULT 1                                  -- Version number for optimistic locking
);

-- Index for fast group retrieval
CREATE INDEX idx_groups_creator_user_id ON groups (creator_user_id);
CREATE INDEX idx_groups_last_activity_at ON groups (last_activity_at);
CREATE INDEX idx_groups_activity_count ON groups (activity_count);

-- +goose Down
DROP INDEX IF EXISTS idx_groups_creator_user_id;
DROP TABLE IF EXISTS groups;
