-- +goose Up
-- Create the enum type for membership roles
CREATE TYPE membership_role AS ENUM ('member', 'admin', 'moderator');
CREATE TABLE group_memberships (
    id BIGSERIAL PRIMARY KEY,                                     -- Unique membership ID
    group_id BIGINT REFERENCES groups(id) ON DELETE CASCADE,       -- Reference to the group
    user_id BIGINT REFERENCES users(id) ON DELETE CASCADE,         -- User who is a member of the group
    status mfa_status_type DEFAULT 'pending',                          -- Status (pending, approved, etc.)
    approval_time TIMESTAMP,                                       -- Time when the request was approved
    request_time TIMESTAMP(0) WITH TIME ZONE DEFAULT NOW(),              -- When the request was made
    role membership_role DEFAULT 'member',                               -- user's role
    created_at TIMESTAMP(0) WITH TIME ZONE DEFAULT NOW(),                -- Membership creation time
    updated_at TIMESTAMP(0) WITH TIME ZONE DEFAULT NOW()                 -- Last update time
);

-- Index for faster lookup of group memberships
CREATE INDEX idx_group_memberships_group_id_user_id ON group_memberships (group_id, user_id);

-- +goose Down
DROP INDEX IF EXISTS idx_group_memberships_group_id_user_id;
DROP TABLE IF EXISTS group_memberships;
