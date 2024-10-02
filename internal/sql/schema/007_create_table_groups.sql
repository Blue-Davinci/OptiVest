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
    version INTEGER DEFAULT 1,                                  -- Version number for optimistic locking
    UNIQUE(name, creator_user_id)
);

-- +goose StatementBegin
-- Trigger function to update the group activity count and last activity timestamp
-- This will be used by by the group based items
CREATE FUNCTION update_group_activity() RETURNS TRIGGER AS $$
BEGIN
    -- Update the activity count and last activity timestamp
    UPDATE groups
    SET activity_count = activity_count + 1,
        last_activity_at = NOW(),
        updated_at = NOW()
    WHERE id = NEW.group_id;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

-- +goose StatementBegin
-- Add the groups creater to the group_memberships table as the admin
CREATE FUNCTION add_creator_to_group_membership() RETURNS TRIGGER AS $$
BEGIN
    -- Insert the creator into group_memberships with role 'admin'
    INSERT INTO group_memberships (group_id, user_id, status, approval_time,role, created_at, updated_at)
    VALUES (NEW.id, NEW.creator_user_id,'accepted', NOW(), 'admin', NOW(), NOW());
    
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

-- +goose StatementBegin
-- Create the trigger on the 'to add the user' to the membership table
CREATE TRIGGER trigger_add_creator_to_memberships
AFTER INSERT ON groups
FOR EACH ROW
EXECUTE FUNCTION add_creator_to_group_membership();
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER trigger_update_goals_tracking_timestamp
BEFORE UPDATE ON groups
FOR EACH ROW
EXECUTE FUNCTION update_timestamp();
-- +goose StatementEnd


-- Index for fast group retrieval
CREATE INDEX idx_groups_creator_user_id ON groups (creator_user_id);
CREATE INDEX idx_groups_last_activity_at ON groups (last_activity_at);
CREATE INDEX idx_groups_activity_count ON groups (activity_count);

-- +goose Down
-- +goose StatementBegin
DROP TRIGGER IF EXISTS trigger_add_creator_to_memberships ON groups;
DROP FUNCTION IF EXISTS add_creator_to_group_membership;
DROP FUNCTION IF EXISTS update_group_activity;
-- +goose StatementEnd
DROP INDEX IF EXISTS idx_groups_creator_user_id;
DROP TABLE IF EXISTS groups;
