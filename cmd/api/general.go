package main

import (
	"fmt"
	"net/http"

	"github.com/Blue-Davinci/OptiVest/internal/data"
	"github.com/Blue-Davinci/OptiVest/internal/validator"
	"go.uber.org/zap"
)

// createContactUsHandler() is a handler function that creates a contact us request for a user with a specific email.
// Check if a user is anonymous. If they are, set the userID to 0 otherwise set it to the user's ID via
// The context value. Then, create a contact us struct and decode the request body into it.
// Call the MapContactUsToConstant method to map the contact us status to a constant.
// We then validate the recieved data and proceed to save the contact us request in the database.
func (app *application) createContactUsHandler(w http.ResponseWriter, r *http.Request) {
	// set the input
	var input struct {
		Name    string `json:"name"`
		Email   string `json:"email"`
		Subject string `json:"subject"`
		Message string `json:"message"`
	}
	// decode the request body into the input struct
	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}
	// check if the user is anonymous
	user := app.contextGetUser(r)
	if user.IsAnonymous() {
		user.ID = 0
	}
	// create a contact us struct
	contactUs := &data.ContactUs{
		Name:    input.Name,
		Email:   input.Email,
		Subject: input.Subject,
		Message: input.Message,
	}
	// map the contact us status to a constant
	status, err := app.models.GeneralManagerModel.MapContactUsToConstant("pending")
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}
	contactUs.Status = status
	// validate the contact us struct
	v := validator.New()
	if data.ValidateContactUs(v, contactUs); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}
	// save the contact us request in the database
	err = app.models.GeneralManagerModel.CreateContactUs(user.ID, contactUs)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	// send acknowledgment Email
	app.background(func() {
		data := map[string]any{
			"name":    contactUs.Name,
			"subject": contactUs.Subject,
			"message": contactUs.Message,
		}
		err = app.mailer.Send(contactUs.Email, "contact_acknowledgment.tmpl", data)
		if err != nil {
			app.logger.Info("background task failed: ", zap.Error(err))
		}
	})
	// send a 201 created response returning a message
	message := fmt.Sprintf("Contact Us request for %s has been received", contactUs.Email)
	err = app.writeJSON(w, http.StatusCreated, envelope{"message": message}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}

}
