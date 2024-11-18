package main

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/Blue-Davinci/OptiVest/internal/data"
	"github.com/Blue-Davinci/OptiVest/internal/validator"
	"github.com/shopspring/decimal"
)

// createNewUserGroupHandler() is a handler function that creates a new user group
// we will take an input from th euser, validate it and then create a new user group
// we will return an updated group with the ID
func (app *application) createNewUserGroupHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name           string `json:"name"`
		IsPrivate      bool   `json:"is_private"`
		MaxMemberCount int    `json:"max_member_count"`
		Description    string `json:"description"`
	}
	// decode the input
	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}
	// create a new group
	group := &data.Group{
		GroupImageURL:  data.DefaultGroupImageURL,
		Name:           input.Name,
		IsPrivate:      input.IsPrivate,
		MaxMemberCount: input.MaxMemberCount,
		Description:    input.Description,
	}
	// validate the group
	v := validator.New()
	if data.ValidateGroup(v, group); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}
	// create a new group
	err = app.models.FinancialGroupManager.CreateNewUserGroup(app.contextGetUser(r).ID, group)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrGroupNameExists):
			v.AddError("name", "a group with this name already exists")
			app.failedValidationResponse(w, r, v.Errors)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	// send the group in the response
	err = app.writeJSON(w, http.StatusCreated, envelope{"group": group}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}

}

// updateUserGroupHandler() is a handler function that updates a user group
// we will take an input from the user, validate it and then update the user group
// we will return an updated group
func (app *application) updateUserGroupHandler(w http.ResponseWriter, r *http.Request) {
	// prepare our input
	var input struct {
		Name           *string `json:"name"`
		GroupImageURL  *string `json:"group_image_url"`
		IsPrivate      *bool   `json:"is_private"`
		MaxMemberCount *int    `json:"max_member_count"`
		Description    *string `json:"description"`
		Version        int     `json:"version"`
	}
	// get the Group's ID from the URL
	groupID, err := app.readIDParam(r, "groupID")
	if err != nil || groupID < 1 {
		app.notFoundResponse(w, r)
		return
	}
	// get the group by the details
	group, err := app.models.FinancialGroupManager.GetGroupById(groupID)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrGeneralRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	// decode the input
	err = app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}
	// check for changes
	if input.Name != nil {
		group.Name = *input.Name
	}
	if input.GroupImageURL != nil {
		group.GroupImageURL = *input.GroupImageURL
	}
	if input.IsPrivate != nil {
		group.IsPrivate = *input.IsPrivate
	}
	if input.MaxMemberCount != nil {
		group.MaxMemberCount = *input.MaxMemberCount
	}
	if input.Description != nil {
		group.Description = *input.Description
	}
	// validate the group
	v := validator.New()
	if data.ValidateGroup(v, group); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}
	// update the group
	err = app.models.FinancialGroupManager.UpdateUserGroup(groupID, app.contextGetUser(r).ID, group)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrGroupNameExists):
			v.AddError("name", "a group with this name already exists")
			app.failedValidationResponse(w, r, v.Errors)
		case errors.Is(err, data.ErrEditConflict):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	// send the group in the response
	err = app.writeJSON(w, http.StatusOK, envelope{"group": group}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}

}

// updateGroupUserRoleHandler() is a handler function that updates a user's role in a group
// we will take the groupID from the URL and an input body which includes the userID and the role
// The updaterUserID will be obtained from the context as we need to verify that the user is an admin or mod
// We will verify that the group exists and the proceed. We will return the updated group
func (app *application) updateGroupUserRoleHandler(w http.ResponseWriter, r *http.Request) {
	// get the group ID from the URL
	groupID, err := app.readIDParam(r, "groupID")
	if err != nil || groupID < 1 {
		app.notFoundResponse(w, r)
		return
	}
	// get the group by the details
	_, err = app.models.FinancialGroupManager.GetGroupById(groupID)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrGeneralRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	// input
	var input struct {
		UserID int64  `json:"user_id"`
		Role   string `json:"role"`
	}
	// decode the input
	err = app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}
	// map the role
	mappedRole, err := app.models.FinancialGroupManager.MapUserRoleTypeToConstant(input.Role)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrInvalidStatusType):
			app.failedValidationResponse(w, r, map[string]string{"role": "invalid role type"})
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	// update the user role
	updatedAt, err := app.models.FinancialGroupManager.UpdateGroupUserRole(groupID, input.UserID, app.contextGetUser(r).ID, mappedRole)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrEditConflict):
			app.editConflictResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	// send a message with the updatedAt time and new role in the response
	message := fmt.Sprintf("user role updated at %s", updatedAt.Format(time.RFC3339))
	err = app.writeJSON(w, http.StatusOK, envelope{"message": message, "role": mappedRole}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}

}

// createNewGroupInvitation() will create a new group invitation for a specific user.
// This group invitation is only for PRIVATE groups, so we will check if the group is private
// If the group is private, we will check if the group has reached its maximum member count
// If the group has reached its maximum member count, we will return an error
// If the group has not reached its maximum member count, we will create a new group invitation
func (app *application) createNewGroupInvitation(w http.ResponseWriter, r *http.Request) {
	// input
	var input struct {
		GroupID          int64  `json:"group_id"`
		InviteeUserEmail string `json:"invitee_user_email"`
	}
	// decode the input
	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}
	// make  a validator
	v := validator.New()
	// check if user is trying to invite themselves
	if app.contextGetUser(r).Email == input.InviteeUserEmail {
		v.AddError("invitee_user_email", "you cannot invite yourself")
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	// get the group by the details
	group, err := app.models.FinancialGroupManager.GetGroupById(input.GroupID)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrGeneralRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	// check if user is Owner of the group
	// for now only the creator of the group can invite users
	if group.CreatorUserID != app.contextGetUser(r).ID {
		app.notFoundResponse(w, r)
		return
	}

	// check if the group is private
	if !group.IsPrivate {
		v.AddError("group_id", "this group is not private")
		app.failedValidationResponse(w, r, v.Errors)
		return
	}
	// check if the group has reached its maximum member count
	isMaxedOut, err := app.models.FinancialGroupManager.CheckIfGroupMembersAreMaxedOut(input.GroupID)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	if isMaxedOut {
		v.AddError("group_id", "this group has reached its maximum member count")
		app.failedValidationResponse(w, r, v.Errors)
		return
	}
	// check if invitee exists
	inviteeUser, err := app.models.Users.GetByEmail(input.InviteeUserEmail, app.config.encryption.key)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrGeneralRecordNotFound):
			v.AddError("invitee_user_email", "this user does not exist")
			app.failedValidationResponse(w, r, v.Errors)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	// map status
	mappedStatus, err := app.models.FinancialGroupManager.MapInvitationInvitationStatusTypeToConstant("pending")
	if err != nil {
		switch {
		case errors.Is(err, data.ErrInvalidStatusType):
			v.AddError("status", "invalid status type")
			app.failedValidationResponse(w, r, v.Errors)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	// create a new group invitation
	groupInvitation := &data.GroupInvitation{
		GroupID:          input.GroupID,
		InviteeUserEmail: input.InviteeUserEmail,
		Status:           mappedStatus,
	}
	// validate the group invitation
	if data.ValidateGroupInvitation(v, groupInvitation); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}
	// delete any existing group invitation that is not pending
	err = app.models.FinancialGroupManager.DeleteNonPendingGroupInvitationsForUser(input.GroupID, input.InviteeUserEmail)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	// create a new group invitation
	err = app.models.FinancialGroupManager.CreateNewGroupInvitation(app.contextGetUser(r).ID, groupInvitation)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrGroupInvitationExists):
			v.AddError("group_id", "this group invitation already exists")
			app.failedValidationResponse(w, r, v.Errors)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	// send the group invitation in the response
	err = app.writeJSON(w, http.StatusCreated, envelope{"group_invitation": groupInvitation}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
	// ToDO: Send notificion to the invitee
	notificationContent := data.NotificationContent{
		Message: fmt.Sprintf("You have been invited to join the group %s, by %s at %s. Follow the link to accept the invitation", group.Name, app.contextGetUser(r).Email, time.Now().Format("2006-01-02 15:04:05")),
		Meta: data.NotificationMeta{
			Url:      fmt.Sprintf("%s?groupID=%d&email=%s", app.config.frontend.groupinvitationurl, group.ID, inviteeUser.Email),
			ImageUrl: group.GroupImageURL,
			Tags:     "group,invitation",
		},
	}
	app.PublishNotificationToRedis(inviteeUser.ID, data.NotificationTypeGroupInvite, notificationContent)
}

// updateGroupInvitationStatusHandler() will update the status of a group invitation
// we will mainly just use it to change the status of the group invitation
func (app *application) updateGroupInvitationStatusHandler(w http.ResponseWriter, r *http.Request) {
	// gey the group invitation ID from the URL
	groupID, err := app.readIDParam(r, "groupID")
	if err != nil || groupID < 1 {
		app.notFoundResponse(w, r)
		return
	}
	// input for the status
	var input struct {
		InviteeEmail string `json:"invitee_email"`
		Status       string `json:"status"`
	}
	// decode the input
	err = app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}
	// get the group invitation by the details
	groupInvitation, err := app.models.FinancialGroupManager.GetGroupInvitationById(groupID, input.InviteeEmail)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrGeneralRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	// the notification exists, lets proceed
	// validate the status by mapping it (pending, accepted or declined)
	mappedStatus, err := app.models.FinancialGroupManager.MapInvitationInvitationStatusTypeToConstant(input.Status)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrInvalidStatusType):
			app.failedValidationResponse(w, r, map[string]string{"status": "invalid status type"})
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	// if mapped status is pending then prevent the user from changing it
	if mappedStatus == data.InviationStatusTypePending {
		app.failedValidationResponse(w, r, map[string]string{"status": "status cannot be pending"})
		return
	}
	// update the group invitation status
	err = app.models.FinancialGroupManager.UpdateGroupInvitationStatus(mappedStatus, groupInvitation)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrEditConflict):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	// make a message depending on whether status was accepted or rejected
	var message string
	if mappedStatus == data.InviationStatusTypeAccepted {
		message = "the group invitation has been accepted"
	} else {
		message = "the group invitation has been rejected"
	}
	// send the group invitation in the response
	err = app.writeJSON(w, http.StatusOK, envelope{"message": message, "group_invitation": groupInvitation}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
	// ToDo: Notify inviter of acceptance
	notificationContent := data.NotificationContent{
		Message: fmt.Sprintf("%s has seen your group invitation and %s", app.contextGetUser(r).Email, message),
		Meta: data.NotificationMeta{
			Url:      fmt.Sprintf("%s/%d", app.config.frontend.groupurl, groupID),
			ImageUrl: "",
			Tags:     "groups, invitation, status",
		},
	}
	app.PublishNotificationToRedis(groupInvitation.InviterUserID, data.NotificationTypeGroupInvite, notificationContent)
}

// createNewGroupGoalHandler() will create a new group goal for a group
// we will take an input from the user, validate it and then create a new group goal
// Some of the validations, apart from data sanity, include whether current amount
// is more than the target amount and that the start date is before the end date
// Groups are meant to be straightforward and simple, so we will not include any
// complex validations
func (app *application) createNewGroupGoalHandler(w http.ResponseWriter, r *http.Request) {
	// input
	var input struct {
		GroupID       int64            `json:"group_id"`
		Name          string           `json:"name"`
		TargetAmount  decimal.Decimal  `json:"target_amount"`
		CurrentAmount decimal.Decimal  `json:"current_amount"`
		StartDate     time.Time        `json:"start_date"`
		EndDate       data.CustomTime1 `json:"end_date"`
	}
	// decode the input
	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}
	// check if the group exists
	_, err = app.models.FinancialGroupManager.GetGroupById(input.GroupID)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrGeneralRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	// create a new group goal
	groupGoal := &data.GroupGoal{
		GroupID:       input.GroupID,
		GoalName:      input.Name,
		TargetAmount:  input.TargetAmount,
		CurrentAmount: input.CurrentAmount,
		CreatedAt:     input.StartDate,
		Deadline:      input.EndDate,
	}
	// make a validator
	v := validator.New()
	// validate the group goal
	if data.ValidateGroupGoal(v, groupGoal); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}
	// create a new group goal
	err = app.models.FinancialGroupManager.CreateNewGroupGoal(app.contextGetUser(r).ID, groupGoal)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrGroupNameExists):
			v.AddError("name", "a group goal with this name already exists")
			app.failedValidationResponse(w, r, v.Errors)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	// send the group goal in the response
	err = app.writeJSON(w, http.StatusCreated, envelope{"group_goal": groupGoal}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// updateGroupGoalHandler() will update a group goal for a group
// This will be a permission route and only the creator of the group or a Group Admin/Moderator
// will be able to update the group goal
func (app *application) updateGroupGoalHandler(w http.ResponseWriter, r *http.Request) {
	// grab group ID from the URL
	groupID, err := app.readIDParam(r, "groupGoalID")
	if err != nil || groupID < 1 {
		app.notFoundResponse(w, r)
		return
	}
	// input
	var input struct {
		Name        *string           `json:"name"`
		EndDate     *data.CustomTime1 `json:"end_date"`
		Description *string           `json:"description"`
	}
	// decode the input
	err = app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}
	// get the group goal by the details
	groupGoal, err := app.models.FinancialGroupManager.GetGroupGoalById(groupID)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrGeneralRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	// CHECK FOR CHANGES
	if input.Name != nil {
		groupGoal.GoalName = *input.Name
	}
	if input.Description != nil {
		groupGoal.Description = *input.Description
	}
	if input.EndDate != nil {
		groupGoal.Deadline = *input.EndDate
	}
	// validate the group goal
	v := validator.New()
	if data.ValidateGroupGoal(v, groupGoal); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}
	// update the group goal
	err = app.models.FinancialGroupManager.UpdateGroupGoal(app.contextGetUser(r).ID, groupGoal)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrGroupNameExists):
			v.AddError("name", "a group goal with this name already exists")
			app.failedValidationResponse(w, r, v.Errors)
		case errors.Is(err, data.ErrEditConflict):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	// send the group goal in the response
	err = app.writeJSON(w, http.StatusOK, envelope{"group_goal": groupGoal}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// createNewGroupTransactionHandler() will create a new group transaction for a group
// we will take an input from the user, validate it and then create a new group transaction
// Any added transaction, will be immediately added to the group's current amount
func (app *application) createNewGroupTransactionHandler(w http.ResponseWriter, r *http.Request) {
	// input
	var input struct {
		GroupID     int64           `json:"group_id"`
		GoalID      int64           `json:"goal_id"`
		Amount      decimal.Decimal `json:"amount"`
		Description string          `json:"description"`
	}
	// decode the input
	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}
	// check if user is member and group exists
	err = app.models.FinancialGroupManager.CheckIfGroupExistsAndUserIsMember(app.contextGetUser(r).ID, input.GroupID)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrGeneralRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	// check if Goal exists
	groupGoal, err := app.models.FinancialGroupManager.GetGroupGoalById(input.GoalID)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrGeneralRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	// check that groupGoal.CurrentAmount is less than groupGoal.TargetAmount
	// if it is not less than, then we will not allow the user to add a new transaction
	if groupGoal.CurrentAmount.GreaterThanOrEqual(groupGoal.TargetAmount) {
		app.failedValidationResponse(w, r, map[string]string{"amount": "goal has already been reached"})
		return
	}
	// amount left to reach the target
	amountLeft := groupGoal.TargetAmount.Sub(groupGoal.CurrentAmount)
	// create a new transaction
	groupTransaction := &data.GroupTransaction{
		GoalID:      input.GoalID,
		Amount:      input.Amount,
		Description: input.Description,
	}
	// make a validator
	v := validator.New()
	// validate the group transaction
	if data.ValidateGroupTransaction(v, groupTransaction); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}
	// create a new group transaction
	err = app.models.FinancialGroupManager.CreateNewGroupTransaction(app.contextGetUser(r).ID, groupTransaction)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrOverFunding):
			message := fmt.Sprintf("this transaction will overfund the goal, the amount you can enter is %s", amountLeft.String())
			v.AddError("amount", message)
			app.failedValidationResponse(w, r, v.Errors)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	// send the group transaction in the response
	err = app.writeJSON(w, http.StatusCreated, envelope{"group_transaction": groupTransaction}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}

}

// deleteGroupTransactionHandler() will delete a group transaction provided the user is the creator
// of the transaction
// We use ErrGeneralRecordNotFound, to see if the deletion was successful
func (app *application) deleteGroupTransactionHandler(w http.ResponseWriter, r *http.Request) {
	// get the group transaction ID from the URL
	groupTransactionID, err := app.readIDParam(r, "groupTransactionID")
	if err != nil || groupTransactionID < 1 {
		app.notFoundResponse(w, r)
		return
	}
	// delete the group transaction
	_, err = app.models.FinancialGroupManager.DeleteGroupTransaction(app.contextGetUser(r).ID, groupTransactionID)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrGeneralRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	// send a message as a response
	err = app.writeJSON(w, http.StatusOK, envelope{"message": "group transaction deleted"}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// createNewGroupExpenseHandler() will create a new group expense for a group
// we will take an input from the user, validate it and then create a new group expense
func (app *application) createNewGroupExpenseHandler(w http.ResponseWriter, r *http.Request) {
	// input
	var input struct {
		GroupID     int64           `json:"group_id"`
		Amount      decimal.Decimal `json:"amount"`
		Description string          `json:"description"`
		Category    string          `json:"category"`
	}
	// decode the input
	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}
	// check if user is member and group exists
	err = app.models.FinancialGroupManager.CheckIfGroupExistsAndUserIsMember(app.contextGetUser(r).ID, input.GroupID)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrGeneralRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	// create a new expense
	groupExpense := &data.GroupExpense{
		GroupID:     input.GroupID,
		Amount:      input.Amount,
		Description: input.Description,
		Category:    input.Category,
	}
	// make a validator
	v := validator.New()
	// validate the group expense
	if data.ValidateGroupExpense(v, groupExpense); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}
	// create a new group expense
	err = app.models.FinancialGroupManager.CreateNewGroupExpense(app.contextGetUser(r).ID, groupExpense)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	// send the group expense in the response
	err = app.writeJSON(w, http.StatusCreated, envelope{"group_expense": groupExpense}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// deleteGroupExpenseHandler() will delete a group expense provided the user is the creator
// of the expense
// We use ErrGeneralRecordNotFound, to see if the deletion was successful
func (app *application) deleteGroupExpenseHandler(w http.ResponseWriter, r *http.Request) {
	// get the group expense ID from the URL
	groupExpenseID, err := app.readIDParam(r, "groupExpenseID")
	if err != nil || groupExpenseID < 1 {
		app.notFoundResponse(w, r)
		return
	}
	// delete the group expense
	_, err = app.models.FinancialGroupManager.DeleteGroupExpense(app.contextGetUser(r).ID, groupExpenseID)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrGeneralRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	// send a message as a response
	err = app.writeJSON(w, http.StatusOK, envelope{"message": "group expense deleted"}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// getAllGroupsCreatedByUserHandler() will get all the groups created by the user
// we will return a list of groups created by the user
func (app *application) getAllGroupsCreatedByUserHandler(w http.ResponseWriter, r *http.Request) {
	// get all the groups created by the user
	groups, err := app.models.FinancialGroupManager.GetAllGroupsCreatedByUser(app.contextGetUser(r).ID)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrGeneralRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	// send the groups in the response
	err = app.writeJSON(w, http.StatusOK, envelope{"groups": groups}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// getAllGroupsUserIsMemberOfHandler() will get all the groups the user is a member of
// we will return a list of groups the user is a member of
func (app *application) getAllGroupsUserIsMemberOfHandler(w http.ResponseWriter, r *http.Request) {
	app.logger.Info("getting all groups user is a member of")
	// get all the groups the user is a member of
	groups, err := app.models.FinancialGroupManager.GetAllGroupsUserIsMemberOf(app.contextGetUser(r).ID)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrGeneralRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	// send the groups in the response
	err = app.writeJSON(w, http.StatusOK, envelope{"groups": groups}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// getGroupTransactionsByGroupIdHandler() will get all the transactions for a group
// This route supports Pagination and a goalID search (which is optional and defaults to 0 to get all transactions)
// We also accept a groupID as well as the userID
func (app *application) getGroupTransactionsByGroupIdHandler(w http.ResponseWriter, r *http.Request) {
	// The Group ID will be provided as a URL param
	groupID, err := app.readIDParam(r, "groupID")
	if err != nil || groupID < 1 {
		app.notFoundResponse(w, r)
		return
	}
	// verify the urlID
	v := validator.New()
	if data.ValidateURLID(v, groupID, "groupID"); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}
	// the goalID and Filters will be provided as Query params
	var input struct {
		GoalID int64 `json:"goal_id"`
		data.Filters
	}
	qs := r.URL.Query()
	input.GoalID = int64(app.readInt(qs, "goalID", 0, v))
	input.Filters.Page = app.readInt(qs, "page", 1, v)
	input.Filters.PageSize = app.readInt(qs, "page_size", 18, v)
	// We don't use any sort for this endpoint
	input.Filters.Sort = app.readString(qs, "", "")
	// None of the sort values are supported for this endpoint
	input.Filters.SortSafelist = []string{"", ""}
	// Perform validation
	if data.ValidateFilters(v, input.Filters); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}
	// get all the transactions for the group
	transactions, metadata, err := app.models.FinancialGroupManager.GetGroupTransactionsByGroupId(app.contextGetUser(r).ID, groupID, input.GoalID, input.Filters)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrGeneralRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	// send the transactions in the response
	err = app.writeJSON(w, http.StatusOK, envelope{"transactions": transactions, "metadata": metadata}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// getGroupExpensesByGroupIdHandler() will get all the expenses for a group
// This route supports Pagination and a category search (which is optional and defaults to "" to get all expenses)
// We also accept a groupID as well as the userID
func (app *application) getGroupExpensesByGroupIdHandler(w http.ResponseWriter, r *http.Request) {
	// The Group ID will be provided as a URL param
	groupID, err := app.readIDParam(r, "groupID")
	if err != nil || groupID < 1 {
		app.notFoundResponse(w, r)
		return
	}
	// verify the urlID
	v := validator.New()
	if data.ValidateURLID(v, groupID, "groupID"); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}
	// the category and Filters will be provided as Query params
	var input struct {
		Category string `json:"category"`
		data.Filters
	}

	qs := r.URL.Query()
	input.Category = app.readString(qs, "name", "")
	input.Filters.Page = app.readInt(qs, "page", 1, v)
	input.Filters.PageSize = app.readInt(qs, "page_size", 18, v)
	// We don't use any sort for this endpoint
	input.Filters.Sort = app.readString(qs, "", "")
	// None of the sort values are supported for this endpoint
	input.Filters.SortSafelist = []string{"", ""}
	// Perform validation
	if data.ValidateFilters(v, input.Filters); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}
	// get all the expenses for the group
	expenses, metadata, err := app.models.FinancialGroupManager.GetGroupExpensesByGroupId(app.contextGetUser(r).ID, groupID, input.Category, input.Filters)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrGeneralRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	// send the expenses in the response
	err = app.writeJSON(w, http.StatusOK, envelope{"expenses": expenses, "metadata": metadata}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// getDetailedGroupByIdHandler() will get a detailed group by the group ID and user ID
// we will return a detailed group by the ID
func (app *application) getDetailedGroupByIdHandler(w http.ResponseWriter, r *http.Request) {
	app.logger.Info("getting detailed group by ID")
	// get the group ID from the URL
	groupID, err := app.readIDParam(r, "groupID")
	if err != nil || groupID < 1 {
		app.notFoundResponse(w, r)
		return
	}
	// verify the urlID
	// validate the post ID
	v := validator.New()
	if data.ValidateURLID(v, groupID, "groupID"); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}
	// get the group by the details
	group, err := app.models.FinancialGroupManager.GetDetailedGroupById(app.contextGetUser(r).ID, groupID)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrGeneralRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	// send the group in the response
	err = app.writeJSON(w, http.StatusOK, envelope{"group": group}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// adminDeleteGroupMemberHandler() is a handler that allows a group's admin to remove/delete
// a user from the group. It's a DELETE endpoint and allows the useer to send the groupID
// {groupID} and the memberID {memberID} in the URL. The admin's ID will be obtained from the
// context.
func (app *application) adminDeleteGroupMemberHandler(w http.ResponseWriter, r *http.Request) {
	// get the groupID and memberID from the url
	groupID, err := app.readIDParam(r, "groupID")
	if err != nil || groupID < 1 {
		app.notFoundResponse(w, r)
		return
	}
	// memberID
	memberID, err := app.readIDParam(r, "memberID")
	if err != nil || memberID < 1 {
		app.notFoundResponse(w, r)
		return
	}
	// verify the urlID's
	// validate the post ID
	v := validator.New()
	if data.ValidateURLID(v, groupID, "groupID"); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}
	// verify memberID
	if data.ValidateURLID(v, memberID, "memberID"); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}
	// perform the Delete Request
	_, err = app.models.FinancialGroupManager.AdminDeleteGroupMember(app.contextGetUser(r).ID, groupID, memberID)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrGeneralRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	// send the response
	err = app.writeJSON(w, http.StatusOK, envelope{"message": "member deleted successfully"}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// userLeaveGroupHandler() allows users to remove/delete themselves from a given group.
// wwe expect the groupID from the URL and a userID from the context
func (app *application) userLeaveGroupHandler(w http.ResponseWriter, r *http.Request) {
	// get the groupID
	groupID, err := app.readIDParam(r, "groupID")
	if err != nil || groupID < 1 {
		app.notFoundResponse(w, r)
		return
	}
	// verify the urlID
	// validate the post ID
	v := validator.New()
	if data.ValidateURLID(v, groupID, "groupID"); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}
	// delete the user from the group
	_, err = app.models.FinancialGroupManager.UserLeaveGroup(app.contextGetUser(r).ID, groupID)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrGeneralRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	// send the response
	err = app.writeJSON(w, http.StatusOK, envelope{"message": "you have successfully left the group"}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}
