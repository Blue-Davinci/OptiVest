package main

import (
	"errors"
	"net/http"

	"github.com/Blue-Davinci/OptiVest/internal/data"
	"github.com/Blue-Davinci/OptiVest/internal/validator"
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
	_, err = app.models.Users.GetByEmail(input.InviteeUserEmail, app.config.encryption.key)
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
	// validate the status by mapping it
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
		message = "group invitation accepted"
	} else {
		message = "group invitation rejected"
	}
	// send the group invitation in the response
	err = app.writeJSON(w, http.StatusOK, envelope{"message": message, "group_invitation": groupInvitation}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}

}
