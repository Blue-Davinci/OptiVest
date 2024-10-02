package data

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"time"

	"github.com/Blue-Davinci/OptiVest/internal/database"
	"github.com/Blue-Davinci/OptiVest/internal/validator"
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

func ValidateGroup(v *validator.Validator, group *Group) {
	ValidateGroupName(v, group.Name)
	ValidateGroupPrivacy(v, group.IsPrivate)
	ValidateGroupMaxMemberCount(v, group.MaxMemberCount)
	ValidateGroupDescription(v, group.Description)
}
func ValidateGroupUpdate(v *validator.Validator, group *Group) {
	ValidateGroupName(v, group.Name)
	ValidateGroupPrivacy(v, group.IsPrivate)
	ValidateGroupMaxMemberCount(v, group.MaxMemberCount)
	ValidateGroupDescription(v, group.Description)
	ValidateGroupVersion(v, group.Version)
}
func ValidateGroupInvitationGroupID(v *validator.Validator, groupID int64) {
	v.Check(groupID > 0, "group_id", "must be greater than 0")
}
func ValidateGroupInvitation(v *validator.Validator, invitation *GroupInvitation) {
	ValidateGroupInvitationGroupID(v, invitation.GroupID)
	ValidateEmail(v, invitation.InviteeUserEmail)
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
