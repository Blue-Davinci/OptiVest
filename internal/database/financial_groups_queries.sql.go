// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.26.0
// source: financial_groups_queries.sql

package database

import (
	"context"
	"database/sql"
	"time"
)

const checkIfGroupExistsAndUserIsMember = `-- name: CheckIfGroupExistsAndUserIsMember :one
SELECT g.id, g.name
FROM groups g
JOIN group_memberships gm ON g.id = gm.group_id
WHERE g.id = $1         -- Check if the group with this ID exists
  AND gm.user_id = $2  -- Check if this user is a member of the group
  AND gm.status = 'accepted'
`

type CheckIfGroupExistsAndUserIsMemberParams struct {
	ID     int64
	UserID sql.NullInt64
}

type CheckIfGroupExistsAndUserIsMemberRow struct {
	ID   int64
	Name string
}

func (q *Queries) CheckIfGroupExistsAndUserIsMember(ctx context.Context, arg CheckIfGroupExistsAndUserIsMemberParams) (CheckIfGroupExistsAndUserIsMemberRow, error) {
	row := q.db.QueryRowContext(ctx, checkIfGroupExistsAndUserIsMember, arg.ID, arg.UserID)
	var i CheckIfGroupExistsAndUserIsMemberRow
	err := row.Scan(&i.ID, &i.Name)
	return i, err
}

const checkIfGroupMembersAreMaxedOut = `-- name: CheckIfGroupMembersAreMaxedOut :one
SELECT COUNT(*) AS member_count, g.max_member_count
FROM group_memberships gm
JOIN groups g ON g.id = gm.group_id
WHERE gm.group_id = $1
GROUP BY g.max_member_count
`

type CheckIfGroupMembersAreMaxedOutRow struct {
	MemberCount    int64
	MaxMemberCount sql.NullInt32
}

func (q *Queries) CheckIfGroupMembersAreMaxedOut(ctx context.Context, groupID sql.NullInt64) (CheckIfGroupMembersAreMaxedOutRow, error) {
	row := q.db.QueryRowContext(ctx, checkIfGroupMembersAreMaxedOut, groupID)
	var i CheckIfGroupMembersAreMaxedOutRow
	err := row.Scan(&i.MemberCount, &i.MaxMemberCount)
	return i, err
}

const createNewGroupExpense = `-- name: CreateNewGroupExpense :one


INSERT INTO group_expenses (
    group_id, 
    member_id, 
    amount, 
    description, 
    category
    )
VALUES ($1, $2, $3, $4, $5)
RETURNING id, created_at, updated_at
`

type CreateNewGroupExpenseParams struct {
	GroupID     sql.NullInt64
	MemberID    sql.NullInt64
	Amount      string
	Description sql.NullString
	Category    sql.NullString
}

type CreateNewGroupExpenseRow struct {
	ID        int64
	CreatedAt sql.NullTime
	UpdatedAt sql.NullTime
}

// Assuming you have a status column for member approval
func (q *Queries) CreateNewGroupExpense(ctx context.Context, arg CreateNewGroupExpenseParams) (CreateNewGroupExpenseRow, error) {
	row := q.db.QueryRowContext(ctx, createNewGroupExpense,
		arg.GroupID,
		arg.MemberID,
		arg.Amount,
		arg.Description,
		arg.Category,
	)
	var i CreateNewGroupExpenseRow
	err := row.Scan(&i.ID, &i.CreatedAt, &i.UpdatedAt)
	return i, err
}

const createNewGroupGoal = `-- name: CreateNewGroupGoal :one
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
RETURNING id, created_at, updated_at
`

type CreateNewGroupGoalParams struct {
	GroupID       int64
	CreatorUserID int64
	GoalName      string
	TargetAmount  string
	CurrentAmount sql.NullString
	StartDate     time.Time
	Deadline      time.Time
	Description   string
}

type CreateNewGroupGoalRow struct {
	ID        int64
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (q *Queries) CreateNewGroupGoal(ctx context.Context, arg CreateNewGroupGoalParams) (CreateNewGroupGoalRow, error) {
	row := q.db.QueryRowContext(ctx, createNewGroupGoal,
		arg.GroupID,
		arg.CreatorUserID,
		arg.GoalName,
		arg.TargetAmount,
		arg.CurrentAmount,
		arg.StartDate,
		arg.Deadline,
		arg.Description,
	)
	var i CreateNewGroupGoalRow
	err := row.Scan(&i.ID, &i.CreatedAt, &i.UpdatedAt)
	return i, err
}

const createNewGroupInvitation = `-- name: CreateNewGroupInvitation :one
INSERT INTO group_invitations (
    group_id, inviter_user_id, invitee_user_email, status) 
VALUES ($1, $2, $3, $4)
RETURNING id, status, sent_at, expiration_date
`

type CreateNewGroupInvitationParams struct {
	GroupID          sql.NullInt64
	InviterUserID    sql.NullInt64
	InviteeUserEmail string
	Status           InvitationStatusType
}

type CreateNewGroupInvitationRow struct {
	ID             int64
	Status         InvitationStatusType
	SentAt         sql.NullTime
	ExpirationDate time.Time
}

func (q *Queries) CreateNewGroupInvitation(ctx context.Context, arg CreateNewGroupInvitationParams) (CreateNewGroupInvitationRow, error) {
	row := q.db.QueryRowContext(ctx, createNewGroupInvitation,
		arg.GroupID,
		arg.InviterUserID,
		arg.InviteeUserEmail,
		arg.Status,
	)
	var i CreateNewGroupInvitationRow
	err := row.Scan(
		&i.ID,
		&i.Status,
		&i.SentAt,
		&i.ExpirationDate,
	)
	return i, err
}

const createNewGroupTransaction = `-- name: CreateNewGroupTransaction :one
INSERT INTO group_transactions (
    goal_id,
    member_id, 
    amount, 
    description
) VALUES 
($1, $2, $3, $4)
RETURNING id, created_at, updated_at
`

type CreateNewGroupTransactionParams struct {
	GoalID      sql.NullInt64
	MemberID    sql.NullInt64
	Amount      string
	Description sql.NullString
}

type CreateNewGroupTransactionRow struct {
	ID        int64
	CreatedAt sql.NullTime
	UpdatedAt sql.NullTime
}

func (q *Queries) CreateNewGroupTransaction(ctx context.Context, arg CreateNewGroupTransactionParams) (CreateNewGroupTransactionRow, error) {
	row := q.db.QueryRowContext(ctx, createNewGroupTransaction,
		arg.GoalID,
		arg.MemberID,
		arg.Amount,
		arg.Description,
	)
	var i CreateNewGroupTransactionRow
	err := row.Scan(&i.ID, &i.CreatedAt, &i.UpdatedAt)
	return i, err
}

const createNewUserGroup = `-- name: CreateNewUserGroup :one
INSERT INTO groups (
    creator_user_id, group_image_url, name, is_private, max_member_count, description
) VALUES 
($1, $2, $3, $4, $5, $6)
RETURNING id, creator_user_id, activity_count, last_activity_at, created_at, updated_at, version
`

type CreateNewUserGroupParams struct {
	CreatorUserID  sql.NullInt64
	GroupImageUrl  string
	Name           string
	IsPrivate      sql.NullBool
	MaxMemberCount sql.NullInt32
	Description    sql.NullString
}

type CreateNewUserGroupRow struct {
	ID             int64
	CreatorUserID  sql.NullInt64
	ActivityCount  sql.NullInt32
	LastActivityAt sql.NullTime
	CreatedAt      sql.NullTime
	UpdatedAt      sql.NullTime
	Version        sql.NullInt32
}

func (q *Queries) CreateNewUserGroup(ctx context.Context, arg CreateNewUserGroupParams) (CreateNewUserGroupRow, error) {
	row := q.db.QueryRowContext(ctx, createNewUserGroup,
		arg.CreatorUserID,
		arg.GroupImageUrl,
		arg.Name,
		arg.IsPrivate,
		arg.MaxMemberCount,
		arg.Description,
	)
	var i CreateNewUserGroupRow
	err := row.Scan(
		&i.ID,
		&i.CreatorUserID,
		&i.ActivityCount,
		&i.LastActivityAt,
		&i.CreatedAt,
		&i.UpdatedAt,
		&i.Version,
	)
	return i, err
}

const deleteGroupExpense = `-- name: DeleteGroupExpense :one
DELETE FROM group_expenses
WHERE id = $1 AND member_id = $2
RETURNING id
`

type DeleteGroupExpenseParams struct {
	ID       int64
	MemberID sql.NullInt64
}

func (q *Queries) DeleteGroupExpense(ctx context.Context, arg DeleteGroupExpenseParams) (int64, error) {
	row := q.db.QueryRowContext(ctx, deleteGroupExpense, arg.ID, arg.MemberID)
	var id int64
	err := row.Scan(&id)
	return id, err
}

const deleteGroupTransaction = `-- name: DeleteGroupTransaction :one
DELETE FROM group_transactions
WHERE id = $1 AND member_id = $2
RETURNING id
`

type DeleteGroupTransactionParams struct {
	ID       int64
	MemberID sql.NullInt64
}

func (q *Queries) DeleteGroupTransaction(ctx context.Context, arg DeleteGroupTransactionParams) (int64, error) {
	row := q.db.QueryRowContext(ctx, deleteGroupTransaction, arg.ID, arg.MemberID)
	var id int64
	err := row.Scan(&id)
	return id, err
}

const getAllGroupsCreatedByUser = `-- name: GetAllGroupsCreatedByUser :many
WITH user_groups AS (
    SELECT g.id, g.creator_user_id, g.group_image_url, g.name, g.is_private, g.max_member_count, g.description, g.activity_count, g.last_activity_at, g.created_at, g.updated_at, g.version
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

SELECT ug.id, ug.creator_user_id, ug.group_image_url, ug.name, ug.is_private, ug.max_member_count, ug.description, ug.activity_count, ug.last_activity_at, ug.created_at, ug.updated_at, ug.version, 
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
LEFT JOIN group_transaction_stats gts ON ug.id = gts.group_id
`

type GetAllGroupsCreatedByUserRow struct {
	ID                      int64
	CreatorUserID           sql.NullInt64
	GroupImageUrl           string
	Name                    string
	IsPrivate               sql.NullBool
	MaxMemberCount          sql.NullInt32
	Description             sql.NullString
	ActivityCount           sql.NullInt32
	LastActivityAt          sql.NullTime
	CreatedAt               sql.NullTime
	UpdatedAt               sql.NullTime
	Version                 sql.NullInt32
	TopMembers              interface{}
	TotalMembers            sql.NullInt64
	LatestMember            interface{}
	TotalPendingInvitations string
	TopGoals                interface{}
	TotalGroupTransactions  string
	LatestTransactionAmount string
}

func (q *Queries) GetAllGroupsCreatedByUser(ctx context.Context, creatorUserID sql.NullInt64) ([]GetAllGroupsCreatedByUserRow, error) {
	rows, err := q.db.QueryContext(ctx, getAllGroupsCreatedByUser, creatorUserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []GetAllGroupsCreatedByUserRow
	for rows.Next() {
		var i GetAllGroupsCreatedByUserRow
		if err := rows.Scan(
			&i.ID,
			&i.CreatorUserID,
			&i.GroupImageUrl,
			&i.Name,
			&i.IsPrivate,
			&i.MaxMemberCount,
			&i.Description,
			&i.ActivityCount,
			&i.LastActivityAt,
			&i.CreatedAt,
			&i.UpdatedAt,
			&i.Version,
			&i.TopMembers,
			&i.TotalMembers,
			&i.LatestMember,
			&i.TotalPendingInvitations,
			&i.TopGoals,
			&i.TotalGroupTransactions,
			&i.LatestTransactionAmount,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const getAllGroupsUserIsMemberOf = `-- name: GetAllGroupsUserIsMemberOf :many
WITH user_groups AS (
    SELECT g.id, g.creator_user_id, g.group_image_url, g.name, g.is_private, g.max_member_count, g.description, g.activity_count, g.last_activity_at, g.created_at, g.updated_at, g.version
    FROM groups g
    JOIN group_memberships gm ON g.id = gm.group_id
    WHERE gm.user_id = $1 AND g.creator_user_id != $1 AND gm.status = 'accepted'
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

SELECT ug.id, ug.creator_user_id, ug.group_image_url, ug.name, ug.is_private, ug.max_member_count, ug.description, ug.activity_count, ug.last_activity_at, ug.created_at, ug.updated_at, ug.version, 
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
       COALESCE(
           (SELECT jsonb_agg(jsonb_build_object('goal_name', tg.goal_name, 'target_amount', tg.target_amount, 'current_amount', tg.current_amount))
            FROM top_goals tg
            WHERE tg.group_id = ug.id AND tg.row_num <= 5), '[]'::jsonb) AS top_goals,
       COALESCE(gts.total_transactions, 0)::NUMERIC AS total_group_transactions,
       COALESCE(gts.latest_transaction_amount, 0)::NUMERIC AS latest_transaction_amount
FROM user_groups ug
LEFT JOIN group_member_stats gms ON ug.id = gms.group_id
LEFT JOIN group_transaction_stats gts ON ug.id = gts.group_id
`

type GetAllGroupsUserIsMemberOfRow struct {
	ID                      int64
	CreatorUserID           sql.NullInt64
	GroupImageUrl           string
	Name                    string
	IsPrivate               sql.NullBool
	MaxMemberCount          sql.NullInt32
	Description             sql.NullString
	ActivityCount           sql.NullInt32
	LastActivityAt          sql.NullTime
	CreatedAt               sql.NullTime
	UpdatedAt               sql.NullTime
	Version                 sql.NullInt32
	TopMembers              interface{}
	TotalMembers            sql.NullInt64
	LatestMember            interface{}
	TopGoals                interface{}
	TotalGroupTransactions  string
	LatestTransactionAmount string
}

func (q *Queries) GetAllGroupsUserIsMemberOf(ctx context.Context, userID sql.NullInt64) ([]GetAllGroupsUserIsMemberOfRow, error) {
	rows, err := q.db.QueryContext(ctx, getAllGroupsUserIsMemberOf, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []GetAllGroupsUserIsMemberOfRow
	for rows.Next() {
		var i GetAllGroupsUserIsMemberOfRow
		if err := rows.Scan(
			&i.ID,
			&i.CreatorUserID,
			&i.GroupImageUrl,
			&i.Name,
			&i.IsPrivate,
			&i.MaxMemberCount,
			&i.Description,
			&i.ActivityCount,
			&i.LastActivityAt,
			&i.CreatedAt,
			&i.UpdatedAt,
			&i.Version,
			&i.TopMembers,
			&i.TotalMembers,
			&i.LatestMember,
			&i.TopGoals,
			&i.TotalGroupTransactions,
			&i.LatestTransactionAmount,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const getGroupById = `-- name: GetGroupById :one
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
WHERE id = $1
`

func (q *Queries) GetGroupById(ctx context.Context, id int64) (Group, error) {
	row := q.db.QueryRowContext(ctx, getGroupById, id)
	var i Group
	err := row.Scan(
		&i.ID,
		&i.CreatorUserID,
		&i.GroupImageUrl,
		&i.Name,
		&i.IsPrivate,
		&i.MaxMemberCount,
		&i.Description,
		&i.ActivityCount,
		&i.LastActivityAt,
		&i.CreatedAt,
		&i.UpdatedAt,
		&i.Version,
	)
	return i, err
}

const getGroupGoalById = `-- name: GetGroupGoalById :one
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
WHERE id = $1
`

func (q *Queries) GetGroupGoalById(ctx context.Context, id int64) (GroupGoal, error) {
	row := q.db.QueryRowContext(ctx, getGroupGoalById, id)
	var i GroupGoal
	err := row.Scan(
		&i.ID,
		&i.GroupID,
		&i.CreatorUserID,
		&i.GoalName,
		&i.TargetAmount,
		&i.CurrentAmount,
		&i.StartDate,
		&i.Deadline,
		&i.Description,
		&i.Status,
		&i.CreatedAt,
		&i.UpdatedAt,
	)
	return i, err
}

const getGroupGoalsByGroupId = `-- name: GetGroupGoalsByGroupId :one
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
WHERE group_id = $1
`

func (q *Queries) GetGroupGoalsByGroupId(ctx context.Context, groupID int64) (GroupGoal, error) {
	row := q.db.QueryRowContext(ctx, getGroupGoalsByGroupId, groupID)
	var i GroupGoal
	err := row.Scan(
		&i.ID,
		&i.GroupID,
		&i.CreatorUserID,
		&i.GoalName,
		&i.TargetAmount,
		&i.CurrentAmount,
		&i.StartDate,
		&i.Deadline,
		&i.Description,
		&i.Status,
		&i.CreatedAt,
		&i.UpdatedAt,
	)
	return i, err
}

const getGroupInvitationById = `-- name: GetGroupInvitationById :one
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
  AND expiration_date > NOW()
`

type GetGroupInvitationByIdParams struct {
	InviteeUserEmail string
	GroupID          sql.NullInt64
}

func (q *Queries) GetGroupInvitationById(ctx context.Context, arg GetGroupInvitationByIdParams) (GroupInvitation, error) {
	row := q.db.QueryRowContext(ctx, getGroupInvitationById, arg.InviteeUserEmail, arg.GroupID)
	var i GroupInvitation
	err := row.Scan(
		&i.ID,
		&i.GroupID,
		&i.InviterUserID,
		&i.InviteeUserEmail,
		&i.Status,
		&i.SentAt,
		&i.RespondedAt,
		&i.ExpirationDate,
	)
	return i, err
}

const updateExpiredGroupInvitations = `-- name: UpdateExpiredGroupInvitations :exec
UPDATE group_invitations
SET status = 'expired'
WHERE status = 'pending' AND expiration_date < NOW()
`

func (q *Queries) UpdateExpiredGroupInvitations(ctx context.Context) error {
	_, err := q.db.ExecContext(ctx, updateExpiredGroupInvitations)
	return err
}

const updateGroupGoal = `-- name: UpdateGroupGoal :one
UPDATE group_goals SET
    goal_name = $1,
    deadline = $2,
    description = $3
WHERE id = $4  
RETURNING updated_at
`

type UpdateGroupGoalParams struct {
	GoalName    string
	Deadline    time.Time
	Description string
	ID          int64
}

func (q *Queries) UpdateGroupGoal(ctx context.Context, arg UpdateGroupGoalParams) (time.Time, error) {
	row := q.db.QueryRowContext(ctx, updateGroupGoal,
		arg.GoalName,
		arg.Deadline,
		arg.Description,
		arg.ID,
	)
	var updated_at time.Time
	err := row.Scan(&updated_at)
	return updated_at, err
}

const updateGroupInvitationStatus = `-- name: UpdateGroupInvitationStatus :one


UPDATE group_invitations
SET status = $1, responded_at = NOW()
WHERE id = $2
RETURNING responded_at
`

type UpdateGroupInvitationStatusParams struct {
	Status InvitationStatusType
	ID     int64
}

// This ensures the invitation hasn't expired.
func (q *Queries) UpdateGroupInvitationStatus(ctx context.Context, arg UpdateGroupInvitationStatusParams) (sql.NullTime, error) {
	row := q.db.QueryRowContext(ctx, updateGroupInvitationStatus, arg.Status, arg.ID)
	var responded_at sql.NullTime
	err := row.Scan(&responded_at)
	return responded_at, err
}

const updateUserGroup = `-- name: UpdateUserGroup :one
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
RETURNING updated_at
`

type UpdateUserGroupParams struct {
	GroupImageUrl  string
	Name           string
	IsPrivate      sql.NullBool
	MaxMemberCount sql.NullInt32
	Description    sql.NullString
	ActivityCount  sql.NullInt32
	LastActivityAt sql.NullTime
	ID             int64
	Version        sql.NullInt32
	CreatorUserID  sql.NullInt64
}

func (q *Queries) UpdateUserGroup(ctx context.Context, arg UpdateUserGroupParams) (sql.NullTime, error) {
	row := q.db.QueryRowContext(ctx, updateUserGroup,
		arg.GroupImageUrl,
		arg.Name,
		arg.IsPrivate,
		arg.MaxMemberCount,
		arg.Description,
		arg.ActivityCount,
		arg.LastActivityAt,
		arg.ID,
		arg.Version,
		arg.CreatorUserID,
	)
	var updated_at sql.NullTime
	err := row.Scan(&updated_at)
	return updated_at, err
}
