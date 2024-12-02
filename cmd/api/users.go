package main

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/Blue-Davinci/OptiVest/internal/data"
	"github.com/Blue-Davinci/OptiVest/internal/database"
	"github.com/Blue-Davinci/OptiVest/internal/validator"
	"go.uber.org/zap"
)

func (app *application) registerUserHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		FirstName     string    `json:"first_name"`
		LastName      string    `json:"last_name"`
		Email         string    `json:"email"`
		Password      string    `json:"password"`
		PhoneNumber   string    `json:"phone_number"`
		DOB           time.Time `json:"dob"`
		Address       string    `json:"address"`
		CountryCode   string    `json:"country_code"`
		CurrencyCode  string    `json:"currency_code"`
		TermsAccepted bool      `json:"terms_accepted"`
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
		ProfileAvatarURL: data.DefaultProfileImage, // Set the default image for the user
	}
	// Fill in the profile completed field
	user.ProfileCompleted = app.isProfileComplete(user)
	app.logger.Info("Profile Completed: ", zap.Bool("ProfileCompleted", user.ProfileCompleted))
	// lets set the password for the user by using the Set method from the password struct
	err = user.Password.Set(input.Password)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	// Perform validation on the user struct before saving the new user
	v := validator.New()
	if data.ValidateUserRegistration(v, user, input.TermsAccepted); !v.Valid() {
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

	// minimize data we send back to the client
	newUser := data.UserSubInfo{
		FirstName:        user.FirstName,
		LastName:         user.LastName,
		ProfileAvatarURL: user.ProfileAvatarURL,
		Activated:        user.Activated,
	}

	//write our 202 response back to the user and check for any errors
	err = app.writeJSON(w, http.StatusAccepted, envelope{"user": newUser}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}

	// send a notification to the user on successful registration
	notificationContent := data.NotificationContent{
		Message: fmt.Sprintf("Welcome %s, your account has been successfully created. Please check your email for activation instructions.", user.FirstName),
		Meta: data.NotificationMeta{
			Url:      app.config.frontend.baseurl,
			ImageUrl: app.config.frontend.applogourl,
			Tags:     "welcome, registration, account",
		},
	}
	// send the notification to the user
	err = app.PublishNotificationToRedis(user.ID, data.NotificationTypeUserRegistration, notificationContent)
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
	// minimize data we send back to the client
	newUser := data.UserSubInfo{
		FirstName:        user.FirstName,
		LastName:         user.LastName,
		ProfileAvatarURL: user.ProfileAvatarURL,
		Activated:        user.Activated,
	}
	// Send the updated user details to the client in a JSON response.
	err = app.writeJSON(w, http.StatusAccepted, envelope{"user": newUser}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
	// Send a welcome notification'
	notificationContent := data.NotificationContent{
		Message: fmt.Sprintf("%s, your account has been successfully activated. Welcome to OptiVest!", user.FirstName),
		Meta: data.NotificationMeta{
			Url:      app.config.frontend.baseurl,
			ImageUrl: app.config.frontend.applogourl,
			Tags:     "welcome, activation, account",
		},
	}
	// send the notification to the user
	app.PublishNotificationToRedis(user.ID, data.NotificationTypeUserWelcome, notificationContent)
}

// updateUserPasswordHandler() Verifies the password reset token and sets a new password for the user.
func (app *application) updateUserPasswordHandler(w http.ResponseWriter, r *http.Request) {
	// Parse and validate the user's new password and password reset token.
	var input struct {
		Password       string `json:"password"`
		TokenPlaintext string `json:"token"`
	}
	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}
	v := validator.New()
	data.ValidatePasswordPlaintext(v, input.Password)
	data.ValidateTokenPlaintext(v, input.TokenPlaintext)
	if !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}
	// Retrieve the details of the user associated with the password reset token,
	// returning an error message if no matching record was found.
	user, err := app.models.Users.GetForToken(data.ScopePasswordReset, input.TokenPlaintext, app.config.encryption.key)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrGeneralRecordNotFound):
			v.AddError("token", "invalid or expired password reset token")
			app.failedValidationResponse(w, r, v.Errors)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	// Set the new password for the user.
	err = user.Password.Set(input.Password)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	// Save the updated user record in our database, checking for any edit conflicts as
	// normal.
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
	// If everything was successful, then delete all password reset tokens for the user.
	err = app.models.Tokens.DeleteAllForUser(data.ScopePasswordReset, user.ID)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	// Succesful, so we send an email for a succesful password reset
	app.background(func() {
		data := map[string]any{
			"firstName": user.FirstName,
			"lastName":  user.LastName,
		}
		// Send the welcome email, passing in the map above as dynamic data.
		err = app.mailer.Send(user.Email, "password_change_acknowledgment.tmpl", data)
		if err != nil {
			app.logger.Error("Error password reset acknowledgment email", zap.String("email", user.Email), zap.Error(err))
		}
	})
	// Send the user a confirmation message.
	env := envelope{"message": "your password was successfully reset"}
	err = app.writeJSON(w, http.StatusOK, env, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// getUserInformationHandler() is responsible for fetching the user's information.
// soo far we will just return the User as obtained from the context.
// we will need to also impliment an account, award and statistic return
func (app *application) getUserInformationHandler(w http.ResponseWriter, r *http.Request) {
	// Get the user from the context
	user := app.contextGetUser(r)
	// get the awards for the user
	awards, err := app.models.AwardManager.GetAllAwardsForUserByID(user.ID)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	// get the account rating and statistics for the user
	accountStats, err := app.models.AlgoManager.GetAccountStatisticsByUserId(user.ID, user.CreatedAt, awards)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	// Send the user back to the client
	err = app.writeJSON(w, http.StatusOK, envelope{"user": user, "awards": awards, "accountStats": accountStats}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// updateUserInformationHandler() is responsible for updating the user's information.
// This handler will accept a JSON request containing the user's new information and
// update the user's record in the database.
// Users can currently be allowed to update their:
// 1) Personal Info which include :- their first name, last name, phone number, profile avatar URL,
// address, country code, currency code
// 2)investment settings which include:- Time horizon and Risk Tolerance.
func (app *application) updateUserInformationHandler(w http.ResponseWriter, r *http.Request) {
	// an input struct to hold all possible changes. This route supports partial updates, so we
	// set pointers to the fields we want to update.
	var input struct {
		FirstName        *string `json:"first_name"`
		LastName         *string `json:"last_name"`
		ProfileAvatarURL *string `json:"profile_avatar_url"`
		PhoneNumber      *string `json:"phone_number"`
		Address          *string `json:"address"`
		CountryCode      *string `json:"country_code"`
		CurrencyCode     *string `json:"currency_code"`
		// Investment Settings
		TimeHorizon   *string `json:"time_horizon"`
		RiskTolerance *string `json:"risk_tolerance"`
	}
	// acquire the user from the context
	user := app.contextGetUser(r)
	// read the JSON request and store it in the input struct
	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}
	// check if the user is trying to update any of the fields
	if input.FirstName != nil {
		user.FirstName = *input.FirstName
	}
	if input.LastName != nil {
		user.LastName = *input.LastName
	}
	if input.ProfileAvatarURL != nil {
		user.ProfileAvatarURL = *input.ProfileAvatarURL
	}
	if input.PhoneNumber != nil {
		user.PhoneNumber = *input.PhoneNumber
	}
	if input.Address != nil {
		user.Address = *input.Address
	}
	if input.CountryCode != nil {
		user.CountryCode = *input.CountryCode
	}
	if input.CurrencyCode != nil {
		user.CurrencyCode = *input.CurrencyCode
	}
	// check if the user is trying to update any of the investment settings
	if input.TimeHorizon != nil {
		// map the string to a time horizon enum using the mapTimeHorizon function
		timeHorizon := app.models.Users.MapTimeHorizonTypeToConstant(*input.TimeHorizon)
		user.TimeHorizon = timeHorizon
	}
	if input.RiskTolerance != nil {
		// map the string to a risk tolerance enum using the mapRiskTolerance function
		riskTolerance := app.models.Users.MapRiskToleranceTypeToConstant(*input.RiskTolerance)
		user.RiskTolerance = database.NullRiskToleranceType{RiskToleranceType: riskTolerance, Valid: true}
	}
	// Perform validation on the user struct before saving the new user
	v := validator.New()
	if data.ValidateUser(v, user); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}
	// update the user in the database
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
	// send the updated user back to the client
	err = app.writeJSON(w, http.StatusOK, envelope{"user": user}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
	// notify the user of the successful update
	notificationContent := data.NotificationContent{
		Message: fmt.Sprintf("%s, your account information has been successfully updated.", user.FirstName),
		Meta: data.NotificationMeta{
			Url:      app.config.frontend.accountsettings, // Direct the user to the account settings page
			ImageUrl: app.config.frontend.applogourl,
			Tags:     "account, update, notification",
		},
	}
	// Send the notification to the user
	app.PublishNotificationToRedis(user.ID, data.NotificationTypeAccount, notificationContent)
}

// logoutUserHandler() is the main endpoint responsible for logging out the user.
// Currently, we will just terminate a user's SSE connection if they have one.
func (app *application) logoutUserHandler(w http.ResponseWriter, r *http.Request) {
	// Get the user from the context
	userID := app.contextGetUser(r).ID
	// use app.RemoveClient to remove the user
	app.RemoveClient(userID)
	// write 200 ok
	err := app.writeJSON(w, http.StatusOK, envelope{"message": "you have been logged out"}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}
