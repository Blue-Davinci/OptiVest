
-- +goose Up
-- Add unique constraint to prevent multiple memberships for the same user in the same group
ALTER TABLE group_memberships
ADD CONSTRAINT unique_group_user_membership UNIQUE (group_id, user_id);

-- +goose Down
ALTER TABLE group_memberships
DROP CONSTRAINT IF EXISTS unique_group_user_membership;
