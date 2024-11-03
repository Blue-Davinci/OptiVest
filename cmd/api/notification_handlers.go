package main

import (
	"database/sql"
	"errors"
	"net/http"
	"time"

	"github.com/Blue-Davinci/OptiVest/internal/data"
	"github.com/Blue-Davinci/OptiVest/internal/validator"
)

// updatedNotificationHandler() is a notification handler responsible for updating a notification's status.
// This is a patch endpoint that takes in a notification id and a status.
// We first attempt to convert the status into a notification status via data.MapNotificationStatusTypeToConst()
// We return a 200 status code if the notification was updated successfully
func (app *application) updatedNotificationHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Status string `json:"status"`
	}
	// read the notification id from the url
	notificationID, err := app.readIDParam(r, "notificationID")
	if err != nil {
		app.notFoundResponse(w, r)
		return
	}
	// decode the request body into the input struct
	err = app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}
	// convert the status into a notification status
	status, err := data.MapNotificationStatusTypeToConst(input.Status)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}
	// update the notification's status
	err = app.models.NotificationManager.UpdateNotificationReadAtAndStatus(
		notificationID,
		sql.NullTime{Time: time.Now(), Valid: true},
		status)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrEditConflict):
			app.editConflictResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	// return a 200 status code
	err = app.writeJSON(w, http.StatusOK, envelope{"notification": "updated successfully"}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// getAllNotificationsByUserIdHandler() is a notification handler responsible for getting all notifications for a user.
// This is a get endpoint which supports both pagination and filtering via the notification_type query parameter.
// We return a 200 status code if the notifications were retrieved successfully
func (app *application) getAllNotificationsByUserIdHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		NotificationType string
		data.Filters
	}
	// validate the query parameters
	v := validator.New()
	// Call r.URL.Query() to get the url.Values map containing the query string data.
	qs := r.URL.Query()
	// get the parameters
	input.NotificationType = qs.Get("name")
	//get the page & pagesizes as ints and set to the embedded struct
	input.Filters.Page = app.readInt(qs, "page", 1, v)
	input.Filters.PageSize = app.readInt(qs, "page_size", 18, v)
	input.Filters.Sort = app.readString(qs, "", "")
	// None of the sort values are supported for this endpoint
	input.Filters.SortSafelist = []string{"", ""}
	// Perform validation
	if data.ValidateFilters(v, input.Filters); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}
	// convert the notification type into a notification type only if it is not empty
	if input.NotificationType != "" {
		_, err := data.MapNotificationStatusTypeToConst(input.NotificationType)
		if err != nil {
			switch {
			// if the notification type is provided and invalid, return a bad request response
			case errors.Is(err, data.ErrInvalidStatusType) && input.NotificationType != "":
				app.badRequestResponse(w, r, err)
				return
			}
		}
	}
	// get all notifications for a user
	notifications, metadata, err := app.models.NotificationManager.GetAllNotificationsByUserId(app.contextGetUser(r).ID, input.NotificationType, input.Filters)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrGeneralRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	// return a 200 status code
	err = app.writeJSON(w, http.StatusOK, envelope{"notifications": notifications, "metadata": metadata}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// deleteNotificationByIdHandler() is a notification handler responsible for deleting a notification by id.
// This is a delete endpoint that takes in a notification id supplied via he URL.
// We return a 200 status code if the notification was deleted successfully
func (app *application) deleteNotificationByIdHandler(w http.ResponseWriter, r *http.Request) {
	// read the notification id from the url
	notificationID, err := app.readIDParam(r, "notificationID")
	if err != nil {
		app.notFoundResponse(w, r)
		return
	}
	// delete the notification by id
	err = app.models.NotificationManager.DeleteNotificationById(notificationID, app.contextGetUser(r).ID)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrGeneralRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	// return a 200 status code
	err = app.writeJSON(w, http.StatusOK, envelope{"notification": "notification deleted successfully"}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// deleteAllNotificationsByUserIdHandler() is a notification handler responsible for deleting all notifications for a user.
// This is a delete endpoint that takes in a user id.
// We return a 200 status code if the notifications were deleted successfully
func (app *application) deleteAllNotificationsByUserIdHandler(w http.ResponseWriter, r *http.Request) {
	// delete all notifications for a user
	err := app.models.NotificationManager.DeleteAllNotificationsByUserId(app.contextGetUser(r).ID)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	// return a 200 status code
	err = app.writeJSON(w, http.StatusOK, envelope{"notification": "all notifications have been deleted successfully"}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}
