package main

import (
	"errors"
	"net/http"
	"time"

	"github.com/Blue-Davinci/OptiVest/internal/data"
	"github.com/Blue-Davinci/OptiVest/internal/validator"
	"go.uber.org/zap"
)

func (app *application) registerUserHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		FirstName    string    `json:"first_name"`
		LastName     string    `json:"last_name"`
		Email        string    `json:"email"`
		Password     string    `json:"password"`
		PhoneNumber  string    `json:"phone_number"`
		DOB          time.Time `json:"dob"`
		Address      string    `json:"address"`
		CountryCode  string    `json:"country_code"`
		CurrencyCode string    `json:"currency_code"`
	}
	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}
	// lets make a new user from the response input
	user := &data.User{
		FirstName:   input.FirstName,
		LastName:    input.LastName,
		Email:       input.Email,
		PhoneNumber: input.PhoneNumber,
		Activated:   false,
		//ProfileCompleted: app.isProfileComplete(user),
		DOB:              input.DOB,
		Address:          input.Address,
		CountryCode:      input.CountryCode,
		CurrencyCode:     input.CurrencyCode,
		ProfileAvatarURL: data.DefaultImage, // Set the default image for the user
	}
	// Fill in the profile completed field
	user.ProfileCompleted = app.isProfileComplete(user)
	// lets set the password for the user by using the Set method from the password struct
	err = user.Password.Set(input.Password)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	// Perform validation on the user struct before saving the new user
	v := validator.New()
	if data.ValidateUser(v, user); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	// insert our user to the DB
	err = app.models.Users.CreateNewUser(user, app.config.encryption.key)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrDuplicateEmail):
			v.AddError("email", "a user with this email address already exists")
			app.failedValidationResponse(w, r, v.Errors)
		default:
			app.serverErrorResponse(w, r, err)
		}

		return
	}
	app.logger.Info("registering a new user", zap.String("email", user.Email), zap.Int("user id", int(user.ID)))
	// After the user record has been created in the database, generate a new activation
	// token for the user.
	token, err := app.models.Tokens.New(user.ID, data.DefaultTokenExpiryTime, data.ScopeActivation)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	app.logger.Info("Token Has Been generated for new user", zap.String("token", token.Plaintext), zap.Int("user id", int(user.ID)))
	app.background(func() {
		// As there are now multiple pieces of data that we want to pass to our email
		// templates, we create a map to act as a 'holding structure' for the data. This
		// contains the plaintext version of the activation token for the user, along
		// with their ID.
		data := map[string]any{
			"activationURL": app.config.frontend.activationurl + token.Plaintext,
			"firstName":     user.FirstName,
			"lastName":      user.LastName,
			"userID":        user.ID,
		}
		// Send the welcome email, passing in the map above as dynamic data.
		err = app.mailer.Send(user.Email, "user_welcome.tmpl", data)
		if err != nil {
			app.logger.Error("Error sending welcome email", zap.String("email", user.Email), zap.Error(err))
		}
	})

	//write our 202 response back to the user and check for any errors
	err = app.writeJSON(w, http.StatusAccepted, envelope{"user": user}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// activateUserHandler() Handles activating a user. Inactive users cannot perform a multitude
// of functions. This handler accepts a JSON request containing a plaintext activation token
// and activates the user associated with the token & the activate scope if that token exists.
func (app *application) activateUserHandler(w http.ResponseWriter, r *http.Request) {
	// Parse the plaintext activation token from the request body.
	var input struct {
		TokenPlaintext string `json:"token"`
	}
	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}
	// Validate the plaintext token provided by the client.
	v := validator.New()
	if data.ValidateTokenPlaintext(v, input.TokenPlaintext); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}
	// Retrieve the details of the user associated with the token using the
	// GetForToken() method. If no matching record is found, then we let the
	// client know that the token they provided is not valid.
	user, err := app.models.Users.GetForToken(data.ScopeActivation, input.TokenPlaintext, app.config.encryption.key)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrGeneralRecordNotFound):
			v.AddError("token", "invalid or expired activation token")
			app.failedValidationResponse(w, r, v.Errors)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	app.logger.Info("User Version: ", zap.Int("Version", int(user.Version)))
	// Update the user's activation status.
	user.Activated = true
	// Save the updated user record in our database, checking for any edit conflicts in
	// the same way that we did for our movie records.
	err = app.models.Users.UpdateUser(user, app.config.encryption.key)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrEditConflict):
			app.editConflictResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	// If everything went successfully, then we delete all activation tokens for the
	// user.
	err = app.models.Tokens.DeleteAllForUser(data.ScopeActivation, user.ID)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	// Succesful, so we send an email for a succesful activation
	app.background(func() {
		// As there are now multiple pieces of data that we want to pass to our email
		// templates, we create a map to act as a 'holding structure' for the data. This
		// contains the plaintext version of the activation token for the user, along
		// with their ID.
		data := map[string]any{
			"loginURL":  app.config.frontend.loginurl,
			"firstName": user.FirstName,
			"lastName":  user.LastName,
		}
		// Send the welcome email, passing in the map above as dynamic data.
		err = app.mailer.Send(user.Email, "user_succesful_activation.tmpl", data)
		if err != nil {
			app.logger.Error("Error sending welcome email", zap.String("email", user.Email), zap.Error(err))
		}
	})
	// Send the updated user details to the client in a JSON response.
	err = app.writeJSON(w, http.StatusOK, envelope{"user": user}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}
