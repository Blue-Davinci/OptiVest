
-- name: CreateNewUserGroup :one
INSERT INTO groups (
    creator_user_id, group_image_url, name, is_private, max_member_count, description
) VALUES 
($1, $2, $3, $4, $5, $6)
RETURNING id, creator_user_id, activity_count, last_activity_at, created_at, updated_at, version;

-- name: GetGroupById :one
SELECT
    id,
    creator_user_id,
    group_image_url,
    name,
    is_private,
    max_member_count,
    description,
    activity_count,
    last_activity_at,
    created_at,
    updated_at,
    version
FROM groups
WHERE id = $1;

-- name: UpdateUserGroup :one
UPDATE groups SET
    group_image_url = $1,
    name = $2,
    is_private = $3,
    max_member_count = $4,
    description = $5,
    activity_count = $6,
    last_activity_at = $7,
    updated_at = NOW(),
    version = version + 1
WHERE
    id = $8 AND version = $9 AND creator_user_id = $10
RETURNING updated_at;

-- name: CheckIfGroupMembersAreMaxedOut :one
SELECT COUNT(*) AS member_count, g.max_member_count
FROM group_memberships gm
JOIN groups g ON g.id = gm.group_id
WHERE gm.group_id = $1
GROUP BY g.max_member_count;

-- name: CreateNewGroupInvitation :one
INSERT INTO group_invitations (
    group_id, inviter_user_id, invitee_user_email, status) 
VALUES ($1, $2, $3, $4)
RETURNING id, status, sent_at, expiration_date;

-- name: GetGroupInvitationById :one
SELECT
    id,
    group_id,
    inviter_user_id,
    invitee_user_email,
    status,
    sent_at,
    responded_at,
    expiration_date
FROM group_invitations 
WHERE invitee_user_email = $1            -- This checks if the invitee matches the user we are checking for.
  AND group_id = $2                   -- This checks if the invitation is for the specific group.
  AND status = 'pending'              -- This ensures that the invitation is still pending.
  AND expiration_date > NOW();        -- This ensures the invitation hasn't expired.


-- name: UpdateGroupInvitationStatus :one
UPDATE group_invitations
SET status = $1, responded_at = NOW()
WHERE id = $2
RETURNING responded_at;

-- name: UpdateExpiredGroupInvitations :exec
UPDATE group_invitations
SET status = 'expired'
WHERE status = 'pending' AND expiration_date < NOW();

-- name: CreateNewGroupGoal :one
INSERT INTO group_goals (
    group_id, 
    creator_user_id, 
    goal_name,
    target_amount, 
    current_amount, 
    start_date,
    deadline, 
    description
) 
VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8
)
RETURNING id, created_at, updated_at;

-- name: UpdateGroupGoal :one
UPDATE group_goals SET
    goal_name = $1,
    deadline = $2,
    description = $3
WHERE id = $4  
RETURNING updated_at;

-- name: GetGroupGoalById :one
SELECT
    id,
    group_id,
    creator_user_id,
    goal_name,
    target_amount,
    current_amount,
    start_date,
    deadline,
    description,
    status,
    created_at,
    updated_at
FROM group_goals
WHERE id = $1;

-- name: GetGroupGoalsByGroupId :one
SELECT
    id,
    group_id,
    creator_user_id,
    goal_name,
    target_amount,
    current_amount,
    start_date,
    deadline,
    description,
    status,
    created_at,
    updated_at
FROM group_goals
WHERE group_id = $1;

-- name: CreateNewGroupTransaction :one
INSERT INTO group_transactions (
    goal_id,
    member_id, 
    amount, 
    description
) VALUES 
($1, $2, $3, $4)
RETURNING id, created_at, updated_at;

-- name: DeleteGroupTransaction :one
DELETE FROM group_transactions
WHERE id = $1 AND member_id = $2
RETURNING id;

-- name: CheckIfGroupExistsAndUserIsMember :one
SELECT g.id, g.name
FROM groups g
JOIN group_memberships gm ON g.id = gm.group_id
WHERE g.id = $1         -- Check if the group with this ID exists
  AND gm.user_id = $2  -- Check if this user is a member of the group
  AND gm.status = 'accepted'; -- Assuming you have a status column for member approval


-- name: CreateNewGroupExpense :one
INSERT INTO group_expenses (
    group_id, 
    member_id, 
    amount, 
    description, 
    category
    )
VALUES ($1, $2, $3, $4, $5)
RETURNING id, created_at, updated_at;

-- name: DeleteGroupExpense :one
DELETE FROM group_expenses
WHERE id = $1 AND member_id = $2
RETURNING id;
