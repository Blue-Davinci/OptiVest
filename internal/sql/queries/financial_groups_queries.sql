
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

-- name: GetAllGroupsCreatedByUser :many
WITH user_groups AS (
    SELECT g.*
    FROM groups g
    WHERE g.creator_user_id = $1 
),

top_members AS (
    SELECT gm.group_id, gm.user_id, u.first_name, gm.role, u.profile_avatar_url,
           ROW_NUMBER() OVER (PARTITION BY gm.group_id ORDER BY gm.request_time DESC) AS row_num
    FROM group_memberships gm
    JOIN users u ON gm.user_id = u.id
    WHERE gm.status = 'accepted'
),

group_member_stats AS (
    SELECT gm.group_id,
           COUNT(*) AS total_members,
           MAX(u.id) AS latest_member_id,
           MAX(u.first_name) AS latest_member_first_name,
           MAX(u.profile_avatar_url) AS latest_member_avatar,
           MAX(gm.role) AS latest_member_role
    FROM group_memberships gm
    JOIN users u ON gm.user_id = u.id
    WHERE gm.status = 'accepted'
    GROUP BY gm.group_id
),

pending_invitations AS (
    SELECT gi.group_id, COUNT(*) AS total_pending_invitations
    FROM group_invitations gi
    WHERE gi.status = 'pending'
    GROUP BY gi.group_id
),

top_goals AS (
    SELECT gg.group_id, gg.goal_name, gg.target_amount, gg.current_amount,
           ROW_NUMBER() OVER (PARTITION BY gg.group_id ORDER BY gg.created_at DESC) AS row_num
    FROM group_goals gg
    WHERE gg.status = 'ongoing'
),

group_transaction_stats AS (
    SELECT gg.group_id,
           COUNT(gt.id) AS total_transactions,
           MAX(gt.amount)::NUMERIC AS latest_transaction_amount
    FROM group_transactions gt
    JOIN group_goals gg ON gt.goal_id = gg.id
    GROUP BY gg.group_id
)

SELECT ug.*, 
       COALESCE(
           (SELECT jsonb_agg(jsonb_build_object('user_id', tm.user_id, 'first_name', tm.first_name, 'role', tm.role, 'profile_avatar_url', tm.profile_avatar_url))
            FROM top_members tm
            WHERE tm.group_id = ug.id AND tm.row_num <= 5), '[]'::jsonb) AS top_members,
       gms.total_members,
       COALESCE(jsonb_build_object(
           'user_id', gms.latest_member_id, 
           'first_name', gms.latest_member_first_name, 
           'role', gms.latest_member_role,
           'profile_avatar_url', gms.latest_member_avatar
       ), '{}'::jsonb) AS latest_member,
       COALESCE(pi.total_pending_invitations, 0)::NUMERIC AS total_pending_invitations,
       COALESCE(
           (SELECT jsonb_agg(jsonb_build_object('goal_name', tg.goal_name, 'target_amount', tg.target_amount, 'current_amount', tg.current_amount))
            FROM top_goals tg
            WHERE tg.group_id = ug.id AND tg.row_num <= 5), '[]'::jsonb) AS top_goals,
       COALESCE(gts.total_transactions, 0)::NUMERIC AS total_group_transactions,
       COALESCE(gts.latest_transaction_amount, 0)::NUMERIC AS latest_transaction_amount
FROM user_groups ug
LEFT JOIN group_member_stats gms ON ug.id = gms.group_id
LEFT JOIN pending_invitations pi ON ug.id = pi.group_id
LEFT JOIN group_transaction_stats gts ON ug.id = gts.group_id;
