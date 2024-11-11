package data

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/Blue-Davinci/OptiVest/internal/database"
	"github.com/Blue-Davinci/OptiVest/internal/validator"
	"github.com/shopspring/decimal"
)

const (
	DefaultGroupImageURL              = "https://images.unsplash.com/photo-1531206715517-5c0ba140b2b8?ixlib=rb-1.2.1&ixid=eyJhcHBfaWQiOjEyMDd9&w=1000&q=80"
	DefualtFinManGroupsContextTimeout = 5 * time.Second
)
const (
	InviationStatusTypePending  = database.InvitationStatusTypePending
	InviationStatusTypeAccepted = database.InvitationStatusTypeAccepted
	InviationStatusTypeDeclined = database.InvitationStatusTypeDeclined
)

var (
	ErrGroupNameExists       = errors.New("group name already exists")
	ErrInvalidStatusType     = errors.New("invalid status type")
	ErrGroupInvitationExists = errors.New("group invitation already exists")
	ErrOverFunding           = errors.New("overfunding is not allowed, please check the amount")
)

type FinancialGroupManagerModel struct {
	DB *database.Queries
}

// Group struct represents a group in the database
type Group struct {
	ID             int64     `json:"id"`
	CreatorUserID  int64     `json:"creator_user_id"`
	GroupImageURL  string    `json:"group_image_url"`
	Name           string    `json:"name"`
	IsPrivate      bool      `json:"is_private"`
	MaxMemberCount int       `json:"max_member_count"`
	Description    string    `json:"description"`
	ActivityCount  int       `json:"activity_count"`
	LastActivityAt time.Time `json:"last_activity_at"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	Version        int       `json:"version"`
}

// GroupGoal struct represents how we group our goals
type GroupGoal struct {
	ID            int64               `json:"id"`
	GroupID       int64               `json:"group_id"`
	CreatorUserID int64               `json:"creator_user_id"`
	GoalName      string              `json:"name"`
	TargetAmount  decimal.Decimal     `json:"target_amount"`
	CurrentAmount decimal.Decimal     `json:"current_amount"`
	Startdate     CustomTime1         `json:"start_date"`
	Deadline      CustomTime1         `json:"deadline"`
	Description   string              `json:"description"`
	Status        database.GoalStatus `json:"status"`
	CreatedAt     time.Time           `json:"created_at"`
	UpdatedAt     time.Time           `json:"updated_at"`
}

// Group Invitation struct represents a group invitation in the database
type GroupInvitation struct {
	ID               int64                         `json:"id"`
	GroupID          int64                         `json:"group_id"`
	InviterUserID    int64                         `json:"inviter_user_id"`
	InviteeUserEmail string                        `json:"invitee_user_email"`
	Status           database.InvitationStatusType `json:"status"`
	SentAt           time.Time                     `json:"sent_at"`
	RespondedAt      time.Time                     `json:"responded_at,omitempty"`
	ExpirationDate   time.Time                     `json:"expiration_date"`
}

// EnrichedGroupTransaction struct represents a group transaction with additional information
type EnrichedGroupTransaction struct {
	GroupTransaction        []*GroupTransaction `json:"group_transaction"`
	TotalTransactionAmount  decimal.Decimal     `json:"total_transaction_amount"`
	LatestTransactionAmount decimal.Decimal     `json:"latest_transaction_amount"`
}

// GroupTransaction struct represents a group transaction in the database
type GroupTransaction struct {
	ID          int64           `json:"id"`
	GoalID      int64           `json:"goal_id"`
	MemberID    int64           `json:"member_id"`
	Amount      decimal.Decimal `json:"amount"`
	Description string          `json:"description"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

// EnrichedExpense struct represents a group expense with additional information
type EnrichedExpense struct {
	GroupExpense        []*GroupExpense `json:"group_expense"`
	TotalGroupExpenses  decimal.Decimal `json:"total_group_expenses"`
	LatestExpenseAmount decimal.Decimal `json:"latest_expense_amount"`
}

// GroupExpense struct represents a group expense in the database
type GroupExpense struct {
	ID          int64           `json:"id"`          // Unique expense ID
	GroupID     int64           `json:"group_id"`    // Reference to the group
	MemberID    int64           `json:"member_id"`   // Reference to the member who made the expense
	Amount      decimal.Decimal `json:"amount"`      // Amount of the expense
	Description string          `json:"description"` // Optional description of the expense
	Category    string          `json:"category"`    // Category of the expense (e.g., 'operations', 'purchase', etc.)
	CreatedAt   time.Time       `json:"created_at"`  // Time when the expense was created
	UpdatedAt   time.Time       `json:"updated_at"`  // Time when the expense was last updated
}

// Enriched Group struct represents a group with additional information
type EnrichedGroup struct {
	Group                   *Group             `json:"group"`
	GroupGoals              []*SampleGroupGoal `json:"group_goals"`
	TotalMembers            int64              `json:"total_members"`
	LatestMember            *GroupMember       `json:"latest_member"`
	GroupMembers            []*GroupMember     `json:"group_members"`
	TotalPendingInvitations decimal.Decimal    `json:"total_pending_invitations"`
	TotalGroupTransactions  decimal.Decimal    `json:"total_group_transactions"`
	LatestTransactionAmount decimal.Decimal    `json:"latest_transaction_amount"`
}

// DetailedGroup struct represents a group with detailed information
type DetailedGroup struct {
	Group                  *Group
	GroupGoals             []*GroupGoal
	GroupMembers           []*GroupMember
	PendingInvitations     []*GroupInvitation
	TotalGroupTransactions decimal.Decimal
	TotalGroupExpenses     decimal.Decimal
}

// SampleGroupGoal struct represents a sample group goal
type SampleGroupGoal struct {
	GoalName      string  `json:"goal_name"`
	TargetAmount  float64 `json:"target_amount"`
	CurrentAmount float64 `json:"current_amount"`
}

// GroupMember holds data for each member within a group.
type GroupMember struct {
	UserID           int64  `json:"user_id"`
	FirstName        string `json:"first_name"`
	Role             string `json:"role"`
	ProfileAvatarURL string `json:"profile_avatar_url"`
}

// MapInvitationInvitationStatusTypeToConstant() maps the invitation status type to a constant
func (m FinancialGroupManagerModel) MapInvitationInvitationStatusTypeToConstant(invitationStatusType string) (database.InvitationStatusType, error) {
	switch invitationStatusType {
	case "pending":
		return InviationStatusTypePending, nil
	case "accepted":
		return InviationStatusTypeAccepted, nil
	case "declined":
		return InviationStatusTypeDeclined, nil
	default:
		return "", ErrInvalidStatusType
	}
}

func ValidateGroupName(v *validator.Validator, name string) {
	v.Check(name != "", "name", "must be provided")
	v.Check(len(name) < 255, "name", "must be between 1 and 255 characters")
}
func ValidateGroupPrivacy(v *validator.Validator, isPrivate bool) {
	v.Check(reflect.TypeOf(isPrivate).Kind() == reflect.Bool, "is_private", "must be a boolean")
}
func ValidateGroupMaxMemberCount(v *validator.Validator, maxMemberCount int) {
	v.Check(maxMemberCount > 0, "max_member_count", "must be greater than 0")
	v.Check(maxMemberCount < 100, "max_member_count", "must be less than 100")
}
func ValidateGroupDescription(v *validator.Validator, description string) {
	v.Check(len(description) < 1000, "description", "must be less than 1000 characters")
}
func ValidateGroupVersion(v *validator.Validator, version int) {
	v.Check(version < 1, "version", "must be greater than 0")
}

// ValidateGroup() validates the group entry
func ValidateGroup(v *validator.Validator, group *Group) {
	ValidateGroupName(v, group.Name)
	ValidateGroupPrivacy(v, group.IsPrivate)
	ValidateGroupMaxMemberCount(v, group.MaxMemberCount)
	ValidateGroupDescription(v, group.Description)
}

// ValidateGroupUpdate() validates the group update
func ValidateGroupUpdate(v *validator.Validator, group *Group) {
	ValidateGroupName(v, group.Name)
	ValidateGroupPrivacy(v, group.IsPrivate)
	ValidateGroupMaxMemberCount(v, group.MaxMemberCount)
	ValidateGroupDescription(v, group.Description)
	ValidateGroupVersion(v, group.Version)
}

// ================================================================================
// Group Goals
// ================================================================================
func ValidateGroupDate(v *validator.Validator, startDate, endDate time.Time) {
	// check start data is less than end date
	v.Check(startDate.Before(endDate), "start_date", "must be before end date")
	// validate deadline is more than now
	v.Check(endDate.After(time.Now()), "deadline", "must be after today")
}
func ValidateGroupAmounts(v *validator.Validator, targetAmount, currentAmount decimal.Decimal) {
	// check if they are provided
	v.Check(targetAmount.String() != "", "target_amount", "must be provided")
	v.Check(currentAmount.String() != "", "current_amount", "must be provided")
	// check if target is more than current
	v.Check(targetAmount.GreaterThan(currentAmount), "target_amount", "must be greater than current amount")
}
func ValidateGroupGoal(v *validator.Validator, goal *GroupGoal) {
	ValidateGroupName(v, goal.GoalName)
	ValidateGroupDate(v, goal.Startdate.Time, goal.Deadline.Time)
	ValidateGroupAmounts(v, goal.TargetAmount, goal.CurrentAmount)
}

// ================================================================================
// Group Invitations
// ================================================================================
func ValidateGroupInvitationGroupID(v *validator.Validator, groupID int64) {
	v.Check(groupID > 0, "group_id", "must be greater than 0")
}
func ValidateGroupInvitation(v *validator.Validator, invitation *GroupInvitation) {
	ValidateGroupInvitationGroupID(v, invitation.GroupID)
	ValidateEmail(v, invitation.InviteeUserEmail)
}

// ================================================================================
// Group Transactions
// ================================================================================
func ValidateAmount(v *validator.Validator, amount decimal.Decimal, keyvalue string) {
	v.Check(amount.String() != "", keyvalue, "must be provided")
	v.Check(amount.GreaterThan(decimal.NewFromInt(0)), keyvalue, "must be greater than 0")
}
func ValidateGroupTransaction(v *validator.Validator, transaction *GroupTransaction) {
	ValidateBudgetDescription(v, transaction.Description)
	ValidateAmount(v, transaction.Amount, "amount")
}

// ================================================================================
// Group Expenses
// ================================================================================
func ValidateGroupExpense(v *validator.Validator, expense *GroupExpense) {
	ValidateGroupName(v, expense.Description)
	ValidateGroupName(v, expense.Category)
	ValidateAmount(v, expense.Amount, "amount")
}

// CheckIfGroupMembersAreMaxedOut() checks if the group has reached its maximum member count
// We will take the group ID and return a boolean and an error
func (m FinancialGroupManagerModel) CheckIfGroupMembersAreMaxedOut(groupID int64) (bool, error) {
	// get our context
	ctx, cancel := contextGenerator(context.Background(), DefualtFinManGroupsContextTimeout)
	defer cancel()
	// check the group
	memberData, err := m.DB.CheckIfGroupMembersAreMaxedOut(ctx, sql.NullInt64{Int64: groupID, Valid: true})
	if err != nil {
		return false, err
	}
	// check if the group is maxed out
	isMaxedOut := memberData.MaxMemberCount.Int32 <= int32(memberData.MemberCount)
	// we are good now
	return isMaxedOut, nil
}

// GetGroupById() retrieves a group by its ID
// We will take the group ID as an argument and return the group and an error
func (m FinancialGroupManagerModel) GetGroupById(groupID int64) (*Group, error) {
	// get our context
	ctx, cancel := contextGenerator(context.Background(), DefualtFinManGroupsContextTimeout)
	defer cancel()
	// get the group
	returnedGroup, err := m.DB.GetGroupById(ctx, groupID)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrGeneralRecordNotFound
		default:
			return nil, err
		}
	}
	// populate our group
	group := populateGroup(returnedGroup)
	// we are good now
	return group, nil
}

// GetAllGroupsCreatedByUser() retrieves all the groups created by a user
// We will take the user ID as an argument and return [] EnrichedGroup and an error
func (m FinancialGroupManagerModel) GetAllGroupsCreatedByUser(userID int64) ([]*EnrichedGroup, error) {
	ctx, cancel := contextGenerator(context.Background(), DefualtFinManGroupsContextTimeout)
	defer cancel()
	// get the groups
	groups, err := m.DB.GetAllGroupsCreatedByUser(ctx, sql.NullInt64{Int64: userID, Valid: true})
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrGeneralRecordNotFound
		default:
			return nil, err
		}
	}
	// check the length
	if len(groups) == 0 {
		return nil, ErrGeneralRecordNotFound
	}
	// create a slice of enriched groups
	var enrichedGroups []*EnrichedGroup
	// loop through the groups
	for _, group := range groups {
		group, err := populateEnrichedGroup(group)
		if err != nil {
			switch {
			case errors.Is(err, ErrTypeConversionError):
				continue
			default:
				return nil, err
			}
		}
		// append to the slice
		enrichedGroups = append(enrichedGroups, group)
	}
	// we are good now
	return enrichedGroups, nil
}

// GetDetailedGroupById() retrieves a group by its ID and the User ID of the user
// This will return an error if the user is not a member of the group
// The aggregated data includes a json array of the users (which will be unmarshalled to the GroupMember struct)
// Pending invitations jsnon array (which will be unmarshalled to the GroupInvitation struct)
// The group goals json array (which will be unmarshalled to the GroupGoal struct)
// And the totals for the total group transactions, total group expenses and the goal_with_most_transactions
func (m FinancialGroupManagerModel) GetDetailedGroupById(userID, groupID int64) (*DetailedGroup, error) {
	// get our context
	ctx, cancel := contextGenerator(context.Background(), DefualtFinManGroupsContextTimeout)
	defer cancel()
	// get the group
	groupDetails, err := m.DB.GetDetailedGroupById(ctx, database.GetDetailedGroupByIdParams{
		ID:     groupID,
		UserID: sql.NullInt64{Int64: userID, Valid: true},
	})
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrGeneralRecordNotFound
		default:
			return nil, err
		}
	}
	// populate our group
	detailedGroup, err := populateDetailedGroup(groupDetails, userID)
	if err != nil {
		return nil, err
	}
	// we are good now
	return detailedGroup, nil
}

// GetAllGroupsUserIsMemberOf() retrieves all the groups a user is a member of
// We will take the user ID as an argument and return [] EnrichedGroup and an error
func (m FinancialGroupManagerModel) GetAllGroupsUserIsMemberOf(userID int64) ([]*EnrichedGroup, error) {
	ctx, cancel := contextGenerator(context.Background(), DefualtFinManGroupsContextTimeout)
	defer cancel()
	// get the groups
	groups, err := m.DB.GetAllGroupsUserIsMemberOf(ctx, sql.NullInt64{Int64: userID, Valid: true})
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrGeneralRecordNotFound
		default:
			return nil, err
		}
	}

	// check the length
	if len(groups) == 0 {
		return nil, ErrGeneralRecordNotFound
	}
	// create a slice of enriched groups
	var enrichedGroups []*EnrichedGroup
	// loop through the groups
	for _, group := range groups {
		enrichedGroup, err := populateEnrichedGroup(group)
		if err != nil {
			switch {
			case errors.Is(err, ErrTypeConversionError):
				continue
			default:
				return nil, err
			}
		}
		// append to the slice
		enrichedGroups = append(enrichedGroups, enrichedGroup)
	}
	// we are good now
	return enrichedGroups, nil
}

// GetGroupTransactionsByGroupId() retrieves all the group transactions by the group ID, user ID and the FILTERs
// We will take the group ID, user ID, and the filters as arguments and return [] EnrichedGroupTransaction, metadata and an error
func (m FinancialGroupManagerModel) GetGroupTransactionsByGroupId(userID, groupID, goalID int64, filters Filters) (*EnrichedGroupTransaction, Metadata, error) {
	// get our context
	ctx, cancel := contextGenerator(context.Background(), DefualtFinManGroupsContextTimeout)
	defer cancel()

	// get the group transactions
	groupTransactionsRows, err := m.DB.GetGroupTransactionsByGroupId(ctx, database.GetGroupTransactionsByGroupIdParams{
		GroupID: groupID,
		Column2: goalID,
		UserID:  sql.NullInt64{Int64: userID, Valid: true},
		Limit:   int32(filters.limit()),
		Offset:  int32(filters.offset()),
	})
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, Metadata{}, ErrGeneralRecordNotFound
		default:
			return nil, Metadata{}, err
		}
	}

	// check the length
	if len(groupTransactionsRows) == 0 {
		return nil, Metadata{}, ErrGeneralRecordNotFound
	}

	// initialize enrichedGroupTransaction to avoid nil dereference
	enrichedGroupTransaction := &EnrichedGroupTransaction{
		GroupTransaction: []*GroupTransaction{},
	}

	// Totals
	totalTransactionCount := 0

	// loop through the group transactions
	for _, groupTransaction := range groupTransactionsRows {
		totalTransactionCount = int(groupTransaction.TotalTransactions)
		enrichedGroupTransaction.TotalTransactionAmount = decimal.RequireFromString(groupTransaction.TotalTransactionAmount)
		enrichedGroupTransaction.LatestTransactionAmount = decimal.RequireFromString(groupTransaction.LatestTransactionAmount)
		populatedTransaction := populateTransactions(groupTransaction)
		// append to the slice
		enrichedGroupTransaction.GroupTransaction = append(enrichedGroupTransaction.GroupTransaction, populatedTransaction)
	}

	// make the metadata
	metadata := calculateMetadata(totalTransactionCount, filters.Page, filters.PageSize)

	// we are good now
	return enrichedGroupTransaction, metadata, nil
}

// GetGroupExpensesByGroupId() retrieves all the group expenses by the group ID, user ID, Optional search by expense category and the FILTERs
// We will take the group ID, user ID, expense category and the filters as arguments and return *EnrichedExpenses, metadata and an error
func (m FinancialGroupManagerModel) GetGroupExpensesByGroupId(userID, groupID int64, category string, filters Filters) (*EnrichedExpense, Metadata, error) {
	// get our context
	ctx, cancel := contextGenerator(context.Background(), DefualtFinManGroupsContextTimeout)
	defer cancel()

	// get the group expenses
	groupExpensesRows, err := m.DB.GetGroupExpensesByGroupId(ctx, database.GetGroupExpensesByGroupIdParams{
		GroupID: sql.NullInt64{Int64: groupID, Valid: true},
		Column2: category,
		UserID:  sql.NullInt64{Int64: userID, Valid: true},
		Limit:   int32(filters.limit()),
		Offset:  int32(filters.offset()),
	})
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, Metadata{}, ErrGeneralRecordNotFound
		default:
			return nil, Metadata{}, err
		}
	}

	// check the length
	if len(groupExpensesRows) == 0 {
		return nil, Metadata{}, ErrGeneralRecordNotFound
	}

	// initialize enrichedGroupTransaction to avoid nil dereference
	enrichedExpense := &EnrichedExpense{
		GroupExpense: []*GroupExpense{},
	}

	// Totals
	totalExpenseCount := 0

	// loop through the group expenses
	for _, groupExpense := range groupExpensesRows {
		totalExpenseCount = int(groupExpense.TotalExpensesCount)
		enrichedExpense.TotalGroupExpenses = decimal.RequireFromString(groupExpense.TotalExpenseAmount)
		enrichedExpense.LatestExpenseAmount = decimal.RequireFromString(groupExpense.LatestExpenseAmount)
		populatedExpense := populateExpenses(groupExpense)
		// append to the slice
		enrichedExpense.GroupExpense = append(enrichedExpense.GroupExpense, populatedExpense)
	}

	// make the metadata
	metadata := calculateMetadata(totalExpenseCount, filters.Page, filters.PageSize)

	// we are good now
	return enrichedExpense, metadata, nil
}

// CreateNewUserGroup() creates a new user group in the database and returns the ID of the new group
// We will take a pointer to the Group struct as an argument and return an error
func (m FinancialGroupManagerModel) CreateNewUserGroup(userID int64, group *Group) error {
	// get our context
	ctx, cancel := contextGenerator(context.Background(), DefualtFinManGroupsContextTimeout)
	defer cancel()
	// insert the data
	groupInformation, err := m.DB.CreateNewUserGroup(ctx, database.CreateNewUserGroupParams{
		CreatorUserID:  sql.NullInt64{Int64: userID, Valid: true},
		GroupImageUrl:  group.GroupImageURL,
		Name:           group.Name,
		IsPrivate:      sql.NullBool{Bool: group.IsPrivate, Valid: true},
		MaxMemberCount: sql.NullInt32{Int32: int32(group.MaxMemberCount), Valid: true},
		Description:    sql.NullString{String: group.Description, Valid: true},
	})
	if err != nil {
		switch {
		case err.Error() == `pq: duplicate key value violates unique constraint "groups_name_creator_user_id_key"`:
			return ErrGroupNameExists
		default:
			return err
		}
	}
	// set the ID of the group
	group.ID = groupInformation.ID
	group.CreatorUserID = groupInformation.CreatorUserID.Int64
	group.ActivityCount = int(groupInformation.ActivityCount.Int32)
	group.LastActivityAt = groupInformation.LastActivityAt.Time
	group.CreatedAt = groupInformation.CreatedAt.Time
	group.UpdatedAt = groupInformation.UpdatedAt.Time
	group.Version = int(groupInformation.Version.Int32)
	// we are good now
	return nil
}

// UpdateUserGroup() updates the user group in the database
// Only Admins can perform this, even though we will have a middleware for this, we
// the update also includes the creator's user ID.
// We expect the group ID, the creator's user ID, and the group struct to be passed in
func (m FinancialGroupManagerModel) UpdateUserGroup(groupID, creatorUserID int64, group *Group) error {
	// get our context
	ctx, cancel := contextGenerator(context.Background(), DefualtFinManGroupsContextTimeout)
	defer cancel()
	// update the data
	updatedAt, err := m.DB.UpdateUserGroup(ctx, database.UpdateUserGroupParams{
		GroupImageUrl:  group.GroupImageURL,
		Name:           group.Name,
		IsPrivate:      sql.NullBool{Bool: group.IsPrivate, Valid: true},
		MaxMemberCount: sql.NullInt32{Int32: int32(group.MaxMemberCount), Valid: true},
		Description:    sql.NullString{String: group.Description, Valid: true},
		ActivityCount:  sql.NullInt32{Int32: int32(group.ActivityCount), Valid: true},
		LastActivityAt: sql.NullTime{Time: group.LastActivityAt, Valid: true},
		ID:             groupID,
		Version:        sql.NullInt32{Int32: int32(group.Version), Valid: true},
		CreatorUserID:  sql.NullInt64{Int64: creatorUserID, Valid: true},
	})
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return ErrEditConflict
		case err.Error() == `pq: duplicate key value violates unique constraint "groups_name_creator_user_id_key"`:
			return ErrGroupNameExists
		default:
			return err
		}
	}
	// set the ID of the group
	group.UpdatedAt = updatedAt.Time
	// we are good now
	return nil
}

// GetGroupInvitationById() retrieves a group invitation by its ID
// We also take in the invitee user email and return the group invitation and an error
func (m FinancialGroupManagerModel) GetGroupInvitationById(groupID int64, inviteeUserEmail string) (*GroupInvitation, error) {
	// get our context
	ctx, cancel := contextGenerator(context.Background(), DefualtFinManGroupsContextTimeout)
	defer cancel()
	// get the group invitation
	groupInvitation, err := m.DB.GetGroupInvitationById(ctx, database.GetGroupInvitationByIdParams{
		InviteeUserEmail: inviteeUserEmail,
		GroupID:          sql.NullInt64{Int64: groupID, Valid: true},
	})
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrGeneralRecordNotFound
		default:
			return nil, err
		}
	}
	// populate our group invitation
	invitation := populateGroupInvitation(groupInvitation)
	// we are good now
	return invitation, nil
}

// UpdateGroupInvitationStatus() updates the status of a group invitation
// We just take the new status and return an error
func (m FinancialGroupManagerModel) UpdateGroupInvitationStatus(newstatusinvitation database.InvitationStatusType, invitation *GroupInvitation) error {
	// get our context
	ctx, cancel := contextGenerator(context.Background(), DefualtFinManGroupsContextTimeout)
	defer cancel()
	// update the data
	respondedAt, err := m.DB.UpdateGroupInvitationStatus(ctx, database.UpdateGroupInvitationStatusParams{
		Status: newstatusinvitation,
		ID:     invitation.ID,
	})
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return ErrEditConflict
		default:
			return err
		}
	}
	// update response time and status
	invitation.RespondedAt = respondedAt.Time
	invitation.Status = newstatusinvitation
	// we are good now
	return nil

}

// CreateNewGroupInvitation() creates a new group invitation for a user
// This allows a group ADMIN/ Moderator to invite users to their group
// and will only work well when a group is private not public.
// We take in a GroupInvitation struct and return an error
func (m FinancialGroupManagerModel) CreateNewGroupInvitation(userID int64, invitation *GroupInvitation) error {
	// get our context
	ctx, cancel := contextGenerator(context.Background(), DefualtFinManGroupsContextTimeout)
	defer cancel()
	// insert the data
	inviteDetail, err := m.DB.CreateNewGroupInvitation(ctx, database.CreateNewGroupInvitationParams{
		GroupID:          sql.NullInt64{Int64: invitation.GroupID, Valid: true},
		InviterUserID:    sql.NullInt64{Int64: userID, Valid: true},
		InviteeUserEmail: invitation.InviteeUserEmail,
		Status:           invitation.Status,
	})
	if err != nil {
		switch {
		case err.Error() == `pq: duplicate key value violates unique constraint "unique_pending_invitation"`:
			return ErrGroupInvitationExists
		default:
			return err
		}
	}
	// fill in the details
	invitation.ID = inviteDetail.ID
	invitation.InviterUserID = userID
	invitation.Status = inviteDetail.Status
	invitation.SentAt = inviteDetail.SentAt.Time
	invitation.ExpirationDate = inviteDetail.ExpirationDate
	// we are good now
	return nil
}

// UpdateExpiredGroupInvitations() updates the status of expired group invitations
// It is used by a cronjob, dauily to update the status of expired group invitations
func (m FinancialGroupManagerModel) UpdateExpiredGroupInvitations() error {
	// get our context
	ctx, cancel := contextGenerator(context.Background(), DefualtFinManGroupsContextTimeout)
	defer cancel()
	// update the data
	err := m.DB.UpdateExpiredGroupInvitations(ctx)
	if err != nil {
		return err
	}
	// we are good now
	return nil
}

// CreateNewGroupGoal() creates a new group goal in the database
// We take in a GroupGoal struct and return an error
func (m FinancialGroupManagerModel) CreateNewGroupGoal(userID int64, groupGoal *GroupGoal) error {
	// get our context
	ctx, cancel := contextGenerator(context.Background(), DefualtFinManGroupsContextTimeout)
	defer cancel()
	// insert the data
	goalDetail, err := m.DB.CreateNewGroupGoal(ctx, database.CreateNewGroupGoalParams{
		GroupID:       groupGoal.GroupID,
		CreatorUserID: userID,
		GoalName:      groupGoal.GoalName,
		TargetAmount:  groupGoal.TargetAmount.String(),
		CurrentAmount: sql.NullString{String: groupGoal.CurrentAmount.String(), Valid: true},
		StartDate:     groupGoal.Startdate.Time,
		Deadline:      groupGoal.Deadline.Time,
		Description:   groupGoal.Description,
	})
	if err != nil {
		switch {
		case err.Error() == `pq: new row for relation "group_goals" violates check constraint "unique_goal_name_per_user_group"`:
			return ErrGroupNameExists
		default:
			return err
		}
	}
	// fill in the details
	groupGoal.ID = goalDetail.ID
	groupGoal.CreatorUserID = userID
	groupGoal.CreatedAt = goalDetail.CreatedAt
	groupGoal.UpdatedAt = goalDetail.UpdatedAt
	// we are good now
	return nil
}

// UpdateGroupGoal() updates the group goal for a specific goal
// This can only be done by the Group's creator or an Admin
// Add this to the require permission group:admin/moderator
// Even if the goals can be updates, only the name, deadline and description can be updated
// This is to prevent any form of fraud and increase transparency in the group
func (m FinancialGroupManagerModel) UpdateGroupGoal(userID int64, groupGoal *GroupGoal) error {
	// get our context
	ctx, cancel := contextGenerator(context.Background(), DefualtFinManGroupsContextTimeout)
	defer cancel()
	// update the data
	updatedAt, err := m.DB.UpdateGroupGoal(ctx, database.UpdateGroupGoalParams{
		GoalName:    groupGoal.GoalName,
		Deadline:    groupGoal.Deadline.Time,
		Description: groupGoal.Description,
		ID:          groupGoal.ID,
	})
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return ErrEditConflict
		case err.Error() == `pq: new row for relation "group_goals" violates check constraint "unique_goal_name_per_user_group"`:
			return ErrGroupNameExists
		default:
			return err
		}
	}
	// Reflect the updated time
	groupGoal.UpdatedAt = updatedAt
	// we are good now
	return nil
}

// GetGroupGoalById() retrieves a group goal by its ID
// We take in the group goal ID and return the group goal and an error
func (m FinancialGroupManagerModel) GetGroupGoalById(groupGoalID int64) (*GroupGoal, error) {
	// get our context
	ctx, cancel := contextGenerator(context.Background(), DefualtFinManGroupsContextTimeout)
	defer cancel()
	// get the group goal
	groupGoal, err := m.DB.GetGroupGoalById(ctx, groupGoalID)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrGeneralRecordNotFound
		default:
			return nil, err
		}
	}
	// populate our group goal
	goal := populateGroupGoal(groupGoal)
	// we are good now
	return goal, nil
}

// CreateNewGroupTransaction() creates a new group transaction in the database
// We take in a GroupTransaction struct and return an error
func (m FinancialGroupManagerModel) CreateNewGroupTransaction(userID int64, transaction *GroupTransaction) error {
	// get our context
	ctx, cancel := contextGenerator(context.Background(), DefualtFinManGroupsContextTimeout)
	defer cancel()
	// insert the data
	transactionDetail, err := m.DB.CreateNewGroupTransaction(ctx, database.CreateNewGroupTransactionParams{
		GoalID:      sql.NullInt64{Int64: transaction.GoalID, Valid: true},
		MemberID:    sql.NullInt64{Int64: userID, Valid: true},
		Amount:      transaction.Amount.String(),
		Description: sql.NullString{String: transaction.Description, Valid: true},
	})
	if err != nil {
		switch {
		case err.Error() == `pq: new row for relation "group_goals" violates check constraint "no_overfunding"`:
			return ErrOverFunding
		default:
			return err
		}

	}
	// fill in the details
	transaction.ID = transactionDetail.ID
	transaction.MemberID = userID
	transaction.CreatedAt = transactionDetail.CreatedAt.Time
	transaction.UpdatedAt = transactionDetail.UpdatedAt.Time
	// we are good now
	return nil
}

// DeleteGroupTransaction() deletes a group transaction by its ID and member_id/user_id of
// the person who created the transaction
// We return the ID of the deleted transaction and an error especially for sql no rows
func (m FinancialGroupManagerModel) DeleteGroupTransaction(userID, transactionID int64) (int64, error) {
	// get our context
	ctx, cancel := contextGenerator(context.Background(), DefualtFinManGroupsContextTimeout)
	defer cancel()
	// delete the data
	deletedTransactionID, err := m.DB.DeleteGroupTransaction(ctx, database.DeleteGroupTransactionParams{
		ID:       transactionID,
		MemberID: sql.NullInt64{Int64: userID, Valid: true},
	})
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return 0, ErrGeneralRecordNotFound
		default:
			return 0, err
		}
	}
	// we are good now
	return deletedTransactionID, nil
}

// CheckIfGroupExistsAndUserIsMember() checks whether a group exists and if the user is a member
// of that group. We take in the user's ID number and the group ID number and return an error
// of the record not found
func (m FinancialGroupManagerModel) CheckIfGroupExistsAndUserIsMember(userID, groupID int64) error {
	// get our context
	ctx, cancel := contextGenerator(context.Background(), DefualtFinManGroupsContextTimeout)
	defer cancel()
	// check the group
	_, err := m.DB.CheckIfGroupExistsAndUserIsMember(ctx, database.CheckIfGroupExistsAndUserIsMemberParams{
		UserID: sql.NullInt64{Int64: userID, Valid: true},
		ID:     groupID,
	})
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return ErrGeneralRecordNotFound
		default:
			return err
		}
	}
	// we are good now
	return nil
}

// CreateNewGroupExpense() creates a new group expense in the database
// We take in pointers to a group expense and userID and return an error if any
func (m FinancialGroupManagerModel) CreateNewGroupExpense(userID int64, expense *GroupExpense) error {
	// get our context
	ctx, cancel := contextGenerator(context.Background(), DefualtFinManGroupsContextTimeout)
	defer cancel()
	// insert the data
	expenseDetail, err := m.DB.CreateNewGroupExpense(ctx, database.CreateNewGroupExpenseParams{
		GroupID:     sql.NullInt64{Int64: expense.GroupID, Valid: true},
		MemberID:    sql.NullInt64{Int64: userID, Valid: true},
		Amount:      expense.Amount.String(),
		Description: sql.NullString{String: expense.Description, Valid: true},
		Category:    sql.NullString{String: expense.Category, Valid: true},
	})
	if err != nil {
		return err
	}
	// fill in the details
	expense.ID = expenseDetail.ID
	expense.MemberID = userID
	expense.CreatedAt = expenseDetail.CreatedAt.Time
	expense.UpdatedAt = expenseDetail.UpdatedAt.Time
	// we are good now
	return nil
}

// DeleteGroupExpense() deletes a group expense by its ID and member_id/user_id of
// the person who created the expense
// We return the ID of the deleted expense and an error especially for sql no rows
func (m FinancialGroupManagerModel) DeleteGroupExpense(userID, expenseID int64) (int64, error) {
	// get our context
	ctx, cancel := contextGenerator(context.Background(), DefualtFinManGroupsContextTimeout)
	defer cancel()
	// delete the data
	deletedExpenseID, err := m.DB.DeleteGroupExpense(ctx, database.DeleteGroupExpenseParams{
		ID:       expenseID,
		MemberID: sql.NullInt64{Int64: userID, Valid: true},
	})
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return 0, ErrGeneralRecordNotFound
		default:
			return 0, err
		}
	}
	// we are good now
	return deletedExpenseID, nil
}

// populateExpenses() populates the group expenses
func populateExpenses(expenseRow interface{}) *GroupExpense {
	switch expense := expenseRow.(type) {
	case database.GetGroupExpensesByGroupIdRow:
		return &GroupExpense{
			ID:          expense.ExpenseID,
			GroupID:     expense.GroupID.Int64,
			MemberID:    expense.MemberID.Int64,
			Amount:      decimal.RequireFromString(expense.Amount),
			Description: expense.Description.String,
			Category:    expense.Category.String,
			CreatedAt:   expense.CreatedAt.Time,
			UpdatedAt:   expense.UpdatedAt.Time,
		}
	default:
		return nil
	}
}

// populateTransactions() populates the group transactions
func populateTransactions(transactionsRow interface{}) *GroupTransaction {
	switch transaction := transactionsRow.(type) {
	case database.GetGroupTransactionsByGroupIdRow:
		return &GroupTransaction{
			ID:          transaction.TransactionID,
			GoalID:      transaction.GoalID.Int64,
			MemberID:    transaction.MemberID.Int64,
			Amount:      decimal.RequireFromString(transaction.Amount),
			Description: transaction.Description.String,
			CreatedAt:   transaction.CreatedAt.Time,
			UpdatedAt:   transaction.UpdatedAt.Time,
		}
	default:
		return nil
	}
}

// populateEnrichedGroup() populates the enriched group
func populateEnrichedGroup(enrichedGroupRow interface{}) (*EnrichedGroup, error) {
	switch group := enrichedGroupRow.(type) {
	case database.GetAllGroupsCreatedByUserRow:
		// get the group goals marshalling them to a group sample goals
		var groupGoals []*SampleGroupGoal
		// type assert topgoals to byte
		topGoals, ok := group.TopGoals.([]byte)
		if !ok {
			return nil, ErrTypeConversionError
		}
		// unmarshal
		err := json.Unmarshal(topGoals, &groupGoals)
		if err != nil {
			return nil, err
		}
		// get the group members
		var groupMembers []*GroupMember
		// type assert topgoals to byte
		topMembers, ok := group.TopMembers.([]byte)
		if !ok {
			return nil, ErrTypeConversionError
		}
		// unmarshal
		err = json.Unmarshal(topMembers, &groupMembers)
		if err != nil {
			return nil, err
		}

		// get latest member
		var latestMember *GroupMember
		// type assert topgoals to byte
		latestMemberByte, ok := group.LatestMember.([]byte)
		if !ok {
			return nil, ErrTypeConversionError
		}
		// unmarshal
		err = json.Unmarshal(latestMemberByte, &latestMember)
		if err != nil {
			return nil, err
		}
		// create an enriched group
		enrichedGroup := &EnrichedGroup{
			Group:                   populateGroup(group),
			GroupGoals:              groupGoals,
			TotalMembers:            group.TotalMembers.Int64,
			LatestMember:            latestMember,
			GroupMembers:            groupMembers,
			TotalPendingInvitations: decimal.RequireFromString(group.TotalPendingInvitations),
			TotalGroupTransactions:  decimal.RequireFromString(group.TotalGroupTransactions),
			LatestTransactionAmount: decimal.RequireFromString(group.LatestTransactionAmount),
		}
		return enrichedGroup, nil
	case database.GetAllGroupsUserIsMemberOfRow:
		var groupGoals []*SampleGroupGoal
		// type assert topgoals to byte
		topGoals, ok := group.TopGoals.([]byte)
		if !ok {
			return nil, ErrTypeConversionError
		}
		// unmarshal
		err := json.Unmarshal(topGoals, &groupGoals)
		if err != nil {
			return nil, err
		}
		// get the group members
		var groupMembers []*GroupMember
		// type assert topgoals to byte
		topMembers, ok := group.TopMembers.([]byte)
		if !ok {
			return nil, ErrTypeConversionError
		}
		// unmarshal
		err = json.Unmarshal(topMembers, &groupMembers)
		if err != nil {
			return nil, err
		}

		// get latest member
		var latestMember *GroupMember
		// type assert topgoals to byte
		latestMemberByte, ok := group.LatestMember.([]byte)
		if !ok {
			return nil, ErrTypeConversionError
		}
		// unmarshal
		err = json.Unmarshal(latestMemberByte, &latestMember)
		if err != nil {
			return nil, err
		}
		// create an enriched group
		enrichedGroup := &EnrichedGroup{
			Group:                   populateGroup(group),
			GroupGoals:              groupGoals,
			TotalMembers:            group.TotalMembers.Int64,
			LatestMember:            latestMember,
			GroupMembers:            groupMembers,
			TotalGroupTransactions:  decimal.RequireFromString(group.TotalGroupTransactions),
			LatestTransactionAmount: decimal.RequireFromString(group.LatestTransactionAmount),
		}
		return enrichedGroup, nil
	default:
		return nil, fmt.Errorf("error in type assertion")
	}

}

func populateGroup(groupRow interface{}) *Group {
	switch group := groupRow.(type) {
	case database.Group:
		return &Group{
			ID:             group.ID,
			CreatorUserID:  group.CreatorUserID.Int64,
			GroupImageURL:  group.GroupImageUrl,
			Name:           group.Name,
			IsPrivate:      group.IsPrivate.Bool,
			MaxMemberCount: int(group.MaxMemberCount.Int32),
			Description:    group.Description.String,
			ActivityCount:  int(group.ActivityCount.Int32),
			LastActivityAt: group.LastActivityAt.Time,
			CreatedAt:      group.CreatedAt.Time,
			UpdatedAt:      group.UpdatedAt.Time,
			Version:        int(group.Version.Int32),
		}
	case database.GetAllGroupsCreatedByUserRow: // database.GetAllGroupsUserIsMemberOfRow
		return &Group{
			ID:             group.ID,
			CreatorUserID:  group.CreatorUserID.Int64,
			GroupImageURL:  group.GroupImageUrl,
			Name:           group.Name,
			IsPrivate:      group.IsPrivate.Bool,
			MaxMemberCount: int(group.MaxMemberCount.Int32),
			Description:    group.Description.String,
			ActivityCount:  int(group.ActivityCount.Int32),
			LastActivityAt: group.LastActivityAt.Time,
			CreatedAt:      group.CreatedAt.Time,
			UpdatedAt:      group.UpdatedAt.Time,
			Version:        int(group.Version.Int32),
		}
	case database.GetAllGroupsUserIsMemberOfRow:
		return &Group{
			ID:             group.ID,
			CreatorUserID:  group.CreatorUserID.Int64,
			GroupImageURL:  group.GroupImageUrl,
			Name:           group.Name,
			IsPrivate:      group.IsPrivate.Bool,
			MaxMemberCount: int(group.MaxMemberCount.Int32),
			Description:    group.Description.String,
			ActivityCount:  int(group.ActivityCount.Int32),
			LastActivityAt: group.LastActivityAt.Time,
			CreatedAt:      group.CreatedAt.Time,
			UpdatedAt:      group.UpdatedAt.Time,
			Version:        int(group.Version.Int32),
		}
	case database.GetDetailedGroupByIdRow:
		return &Group{
			ID:             group.ID,
			CreatorUserID:  group.CreatorUserID.Int64,
			GroupImageURL:  group.GroupImageUrl,
			Name:           group.Name,
			IsPrivate:      group.IsPrivate.Bool,
			MaxMemberCount: int(group.MaxMemberCount.Int32),
			Description:    group.Description.String,
			ActivityCount:  int(group.ActivityCount.Int32),
			LastActivityAt: group.LastActivityAt.Time,
			CreatedAt:      group.CreatedAt.Time,
			UpdatedAt:      group.UpdatedAt.Time,
			Version:        int(group.Version.Int32),
		}
	default:
		return nil
	}
}

func populateGroupInvitation(invitationRow interface{}) *GroupInvitation {
	switch invitation := invitationRow.(type) {
	case database.GroupInvitation:
		return &GroupInvitation{
			ID:               invitation.ID,
			GroupID:          invitation.GroupID.Int64,
			InviterUserID:    invitation.InviterUserID.Int64,
			InviteeUserEmail: invitation.InviteeUserEmail,
			Status:           invitation.Status,
			SentAt:           invitation.SentAt.Time,
			RespondedAt:      invitation.RespondedAt.Time,
			ExpirationDate:   invitation.ExpirationDate,
		}
	default:
		return nil
	}
}

func populateGroupGoal(groupGoalRow interface{}) *GroupGoal {
	switch groupGoal := groupGoalRow.(type) {
	case database.GroupGoal:
		return &GroupGoal{
			ID:            groupGoal.ID,
			GroupID:       groupGoal.GroupID,
			CreatorUserID: groupGoal.CreatorUserID,
			GoalName:      groupGoal.GoalName,
			TargetAmount:  decimal.RequireFromString(groupGoal.TargetAmount),
			CurrentAmount: decimal.RequireFromString(groupGoal.CurrentAmount.String),
			Startdate:     CustomTime1{groupGoal.StartDate},
			Deadline:      CustomTime1{groupGoal.Deadline},
			Description:   groupGoal.Description,
			Status:        groupGoal.Status,
			CreatedAt:     groupGoal.CreatedAt,
			UpdatedAt:     groupGoal.UpdatedAt,
		}
	default:
		return nil
	}
}

// populateDetailedGroup() populates the detailed group struct
// We receive an interface and return a pointer to a DetailedGroup struct
// For the group goals, group members and pending invitations, we will
// unmarshal the json array to the respective structs
func populateDetailedGroup(groupDetails interface{}, userID int64) (*DetailedGroup, error) {
	switch groupDetails := groupDetails.(type) {
	case database.GetDetailedGroupByIdRow:
		// get the group goals marshalling them to a group sample goals
		var groupGoals []*GroupGoal
		// type assert topgoals to byte
		topGoals, ok := groupDetails.Goals.([]byte)
		if !ok {
			return nil, ErrTypeConversionError
		}
		// unmarshal
		err := json.Unmarshal(topGoals, &groupGoals)
		if err != nil {
			return nil, err
		}
		// get the goal with the most transactions
		var goalWithMostTransactions *GroupGoal
		// type assert topgoals to byte
		goalWithMostTransactionsByte, ok := groupDetails.GoalWithMostTransactions.([]byte)
		if !ok {
			return nil, ErrTypeConversionError
		}
		// unmarshal
		err = json.Unmarshal(goalWithMostTransactionsByte, &goalWithMostTransactions)
		if err != nil {
			return nil, err
		}
		// get the group members
		var groupMembers []*GroupMember
		// type assert topgoals to byte
		topMembers, ok := groupDetails.Members.([]byte)
		if !ok {
			return nil, ErrTypeConversionError
		}
		// unmarshal
		err = json.Unmarshal(topMembers, &groupMembers)
		if err != nil {
			return nil, err
		}
		var userRole string
		// get the user's role
		for _, member := range groupMembers {
			if member.UserID == userID {
				userRole = member.Role
			}
		}

		// get the pending invitations
		var pendingInvitations []*GroupInvitation
		// type assert pending invitations to byte
		pendingInvitationsByte, ok := groupDetails.PendingInvitations.([]byte)
		if !ok {
			return nil, ErrTypeConversionError
		}
		// unmarshal
		err = json.Unmarshal(pendingInvitationsByte, &pendingInvitations)
		if err != nil {
			return nil, err
		}
		// populate our group
		group := populateGroup(groupDetails)
		// get the totals
		totalGroupTransactions := decimal.RequireFromString(groupDetails.TotalGroupTransactions)
		totalGroupExpenses := decimal.RequireFromString(groupDetails.TotalGroupExpenses)
		// we are good now
		// if the user's role is an admin or moderator, we will return everything
		// otherwise, we will return all except the pending invitations

		var detailedGroup *DetailedGroup
		if userRole == "admin" || userRole == "moderator" {
			detailedGroup = &DetailedGroup{
				Group:                  group,
				GroupGoals:             groupGoals,
				GroupMembers:           groupMembers,
				PendingInvitations:     pendingInvitations,
				TotalGroupTransactions: totalGroupTransactions,
				TotalGroupExpenses:     totalGroupExpenses,
			}
		} else {
			detailedGroup = &DetailedGroup{
				Group:                  group,
				GroupGoals:             groupGoals,
				GroupMembers:           groupMembers,
				TotalGroupTransactions: totalGroupTransactions,
				TotalGroupExpenses:     totalGroupExpenses,
			}
		}
		return detailedGroup, nil
	default:
		return nil, nil
	}

}
