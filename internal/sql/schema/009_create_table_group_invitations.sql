
-- +goose Up
CREATE TYPE invitation_status_type AS ENUM ('pending', 'accepted', 'declined', 'expired');

CREATE TABLE group_invitations (
    id BIGSERIAL PRIMARY KEY,                                       -- Unique invitation ID
    group_id BIGINT REFERENCES groups(id) ON DELETE CASCADE,        -- Group reference
    inviter_user_id BIGINT REFERENCES users(id) ON DELETE CASCADE,  -- User who sent the invitation
    invitee_user_email CITEXT NOT NULL REFERENCES users(email) ON DELETE CASCADE,                            -- id of the user being invited
    status invitation_status_type NOT NULL DEFAULT 'pending',                -- Invitation status
    sent_at TIMESTAMP(0) WITH TIME ZONE DEFAULT NOW(),                              -- Invitation sent time
    responded_at TIMESTAMP(0) WITH TIME ZONE,                                       -- When the invitee responded
    expiration_date TIMESTAMP(0) WITH TIME ZONE NOT NULL DEFAULT NOW() + INTERVAL '7 days',   -- Expiration time (default 7 days)
    CONSTRAINT unique_pending_invitation UNIQUE (group_id, invitee_user_email, status)
);

-- +goose StatementBegin
CREATE FUNCTION add_user_to_group_membership() RETURNS TRIGGER AS $$
BEGIN
    IF NEW.status = 'accepted' THEN
        -- Insert into group_memberships with the necessary column list and correct SELECT
        INSERT INTO group_memberships (group_id, user_id, status, role, approval_time, request_time)
        SELECT NEW.group_id, u.id, 'accepted', 'member', NOW(), NOW() 
        FROM users u
        WHERE u.email = NEW.invitee_user_email;
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER trigger_add_user_to_group_membership
AFTER UPDATE OF status ON group_invitations
FOR EACH ROW
WHEN (NEW.status = 'accepted')
EXECUTE FUNCTION add_user_to_group_membership();
-- +goose StatementEnd


-- Indexes for optimization
CREATE INDEX idx_group_invitations_group_id_status ON group_invitations (group_id, status);
CREATE INDEX idx_group_invitations_invitee_id_status ON group_invitations (invitee_user_email, status);

-- +goose Down
DROP INDEX IF EXISTS idx_group_invitations_group_id_status;
DROP INDEX IF EXISTS idx_group_invitations_invitee_id_status;
DROP TRIGGER IF EXISTS trigger_add_user_to_group_membership ON group_invitations;
DROP FUNCTION IF EXISTS add_user_to_group_membership();
DROP TABLE IF EXISTS group_invitations;
DROP TYPE IF EXISTS invitation_status_type;
