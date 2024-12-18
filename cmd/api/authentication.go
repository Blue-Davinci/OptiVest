package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Blue-Davinci/OptiVest/internal/data"
	"github.com/Blue-Davinci/OptiVest/internal/validator"
	"go.uber.org/zap"
)

// createAuthenticationApiKeyHandler() is the main endpoint responsible for creating a new authentication
// token for the user. This endpoint is used when the user wants to authenticate their account.
// We accept a users email and password, validate them, and then check if the user exists in the database.
// If the user exists, we then check if the password matches the one in the database. If the password
// matches, we then generate a new api key with a 72-hour expiry time and the scope 'authentication'.
func (app *application) createAuthenticationApiKeyHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	//read the data from the request
	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}
	// validate the user's password & email
	v := validator.New()
	data.ValidateEmail(v, input.Email)
	data.ValidatePasswordPlaintext(v, input.Password)
	if !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}
	// get the user from the database
	user, err := app.models.Users.GetByEmail(input.Email, app.config.encryption.key)
	if err != nil {
		switch {
		// if the user is not found, we return an invalid credentials response
		case errors.Is(err, data.ErrGeneralRecordNotFound):
			app.invalidCredentialsResponse(w, r)
		default:
			// otherwsie return a 500 internal server error
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	// if the user is not activated, we return an error
	if !user.Activated {
		app.inactiveAccountResponse(w, r)
		return
	}
	// check if the password matches
	match, err := user.Password.Matches(input.Password)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	// if password doesn't match then we shout
	if !match {
		app.invalidCredentialsResponse(w, r)
		return
	}
	// If login is okay, we check if the user has MFA/2FA enabled.
	// If enabled, we divert the flow to another handler that will generate a new MFA token
	// with a different scope and send it back to the user for TOTP auth
	if user.MFAEnabled {
		// TOTP
		app.performMFAOnLogin(w, r, user)
	} else {
		// Otherwise, if the password is correct, we generate a new api_key with a 72-hour
		// expiry time and the scope 'authentication', saving it to the DB
		app.generateAuthenticationTokenAndLogin(user, 72*time.Hour, data.ScopeAuthentication, w, r)
	}
}

// performMFAOnLogin() is a helper that performs MFA on login. We start by checking if there
// is a pending MFA login session for the specific user by checking for the RedisMFALoginPendingPrefixn key,
// if there is, we return an error, otherwise we proceed and generate a session token which we encrypt
// for security reasons. We then generate a TOTP qr url,  save the encrypted token to redis as a value with the RedisMFALoginPendingPrefix
// as the key. We then send the user the encrypted token and the QR code for the user to scan. The user will then
// send the token back to us in addition to the TOTP code to validate their login.
func (app *application) performMFAOnLogin(w http.ResponseWriter, r *http.Request, user *data.User) {
	// Decode our key
	key, err := data.DecodeEncryptionKey(app.config.encryption.key)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	// make the REDIS key
	redisKey := fmt.Sprintf("%s:%d", data.RedisMFALoginPendingPrefix, user.ID)
	// check if there is an existing pending MFA setup for the user
	mfaSession, err := getFromCache[*data.MFASession](context.Background(), app.RedisDB, redisKey)
	if err != nil {
		switch {
		case errors.Is(err, ErrNoDataFoundInRedis):
			// no data found in redis so we can proceed
			// do nothing
		default:
			app.serverErrorResponse(w, r, err)
			return
		}
	}
	// if there is an existing pending session, we return an error
	if mfaSession != nil {
		app.badRequestResponse(w, r, data.ErrRedisMFAKeyAlreadyExists)
		return
	}
	// Generate a token with Scope mfa-login which will be used as a validation token
	// and stored in redis as the value to our key. We will also send it to the user and
	// require the user to send it back to us to validate their login
	mfaToken, err := app.models.Tokens.New(user.ID, 5*time.Minute, data.ScopeMFALogin)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	// encrypt the token
	encryptedToken, err := data.EncryptData(mfaToken.Plaintext, key)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	// generate our TOTP token using the encrypted Token as the value
	// for the key we will use the RedisMFALoginPendingPrefix
	_, err = app.totpTokenGenerator(user.Email, redisKey, mfaToken.Plaintext)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	app.logger.Info("MFA login totp generated with this key", zap.String("plain text saved", mfaToken.Plaintext), zap.String("user email", user.Email))
	app.logger.Info(("MFA Login, we use the following user secret"), zap.String("secret", user.MFASecret), zap.String("redis key", redisKey))
	// we will now send the user the encrypted token and the email
	// returning a 403 Forbidden status code
	err = app.writeJSON(w, http.StatusForbidden, envelope{
		"message":    "Multi-factor authentication is required to proceed.",
		"totp_token": encryptedToken,
		"email":      user.Email,
	}, nil)

	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// generateAuthenticationTokenAndLogin() is a helper that generates a new authentication token
// with a specific expiry time and scope, and then sends it to the user in the response.
// This function serves as the final actor in the login process. Both an MFA login and a none
// MFA login will end up here. As of now, we use a 72hr expiration. You can change the expiration
// time to whatever you want from the caller.
func (app *application) generateAuthenticationTokenAndLogin(user *data.User, timeToLeave time.Duration, scope string, w http.ResponseWriter, r *http.Request) {
	// Generate a new authentication token with a 72-hour expiry time and the scope 'authentication'.
	bearer_token, err := app.models.Tokens.New(user.ID, timeToLeave, scope)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	// Encode the authentication token to JSON and send it in the response.
	// Encode the apikey to json and send it to the user with a 201 Created status code
	err = app.writeJSON(w, http.StatusCreated, envelope{
		"api_key": bearer_token,
		"user": map[string]string{
			"id":                strconv.Itoa(int(user.ID)),
			"first_name":        user.FirstName,
			"last_name":         user.LastName,
			"user_role":         user.UserRole,
			"profile_url":       user.ProfileAvatarURL,
			"profile_completed": fmt.Sprintf("%t", user.ProfileCompleted),
			"country_code":      user.CountryCode,
			"currency_code":     user.CurrencyCode,
		},
	}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// validateMFALoginAttemptHandler() is a handler method that verifies the MFA login attempt.
// We expect the user to send back the encrypted token and the TOTP code they received in the body.
// We first check if the user has mfa enable, if not we send back an error. We then check if the user
// has a pending MFA login session in redis, if not we send back an error for them to try and login again.
// We then decrypt the token and check if it matches the one we have in redis, if not we send back an error.
// We then validate the TOTP code, if it's correct, we invoke generateAuthenticationTokenAndLogin() to
// generate the bearer token and proceed.
func (app *application) validateMFALoginAttemptHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		TOTPToken string `json:"totp_token"`
		TOTPCode  string `json:"totp_code"`
		Email     string `json:"email"`
	}
	// IF THEY DO, we read the body into the input struct
	// read the body into the input struct
	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}
	// make an mfatoken
	mfaToken := &data.MFAToken{
		TOTPCode:  input.TOTPCode,
		TOTPToken: input.TOTPToken,
		Email:     input.Email,
	}
	// validate the input
	v := validator.New()
	// validate the input
	if data.ValidateTOTPCode(v, mfaToken); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}
	// get the user from the database
	user, err := app.models.Users.GetByEmail(input.Email, app.config.encryption.key)
	if err != nil {
		switch {
		// if the user is not found, we return an invalid credentials response
		case errors.Is(err, data.ErrGeneralRecordNotFound):
			app.invalidCredentialsResponse(w, r)
		default:
			// otherwsie return a 500 internal server error
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	// make redis key
	redisKey := fmt.Sprintf("%s:%d", data.RedisMFALoginPendingPrefix, user.ID)
	// check if user has a pending MFA login session, if not we return an error
	mfaSession, err := getFromCache[*data.MFASession](context.Background(), app.RedisDB, redisKey)
	if err != nil {
		switch {
		case errors.Is(err, ErrNoDataFoundInRedis):
			// return error
			app.sessionExpiredResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	// Decode our key
	encryption_key, err := data.DecodeEncryptionKey(app.config.encryption.key)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	// decrypt the token
	decryptedToken, err := data.DecryptData(mfaToken.TOTPToken, encryption_key)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	app.logger.Info("MFA setup pending status decrypted token", zap.String("decryptedToken", decryptedToken), zap.String("stored token", (*mfaSession).Value))
	app.logger.Info("Recieved TOTP Code", zap.String("TOTPCode", mfaToken.TOTPCode), zap.String("Recieved TOTPToken", mfaToken.TOTPToken))

	// check if the decrypted token matches the one in redis
	if decryptedToken != (*mfaSession).Value {
		app.invalidCredentialsResponse(w, r)
		return
	}
	// validate the TOTP code
	// Verify the code and delete the secret, if there is an error, we abort
	app.logger.Info(("MFA Login Verification, we use the following user secret"), zap.String("secret", user.MFASecret))
	err = app.validateAndDeleteTOTP(mfaToken.TOTPCode, user.MFASecret, redisKey)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrInvalidTOTPCode):
			app.badRequestResponse(w, r, err)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	// Update the users last MFA login time
	now := time.Now()
	user.MFALastChecked = &now
	// Save the user to the DB
	err = app.models.Users.UpdateUser(user, app.config.encryption.key)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	// everything is okay, we generate a new authentication token
	app.generateAuthenticationTokenAndLogin(user, 72*time.Hour, data.ScopeAuthentication, w, r)
}

// createPasswordResetTokenHandler() Generates a password reset token and send it to the user's email address.
// This endpoint is used when the user wants to reset their password. We accept the user's email address,
// validate it, and then check if the user exists in the database. If the user exists, we then check if the
// user's account is activated. If the account is activated, we create a new password reset token with a 45-minute
// expiry time and send it to the user's email address.
func (app *application) createPasswordResetTokenHandler(w http.ResponseWriter, r *http.Request) {
	// Parse and validate the user's email address.
	var input struct {
		Email string `json:"email"`
		// ToDo: Add TOTP Code Here only for users with MFA enabled
		TOTPCode string `json:"totp_code"`
	}
	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}
	v := validator.New()
	if data.ValidateEmail(v, input.Email); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}
	// Try to retrieve the corresponding user record for the email address. If it can't
	// be found, return an error message to the client.
	user, err := app.models.Users.GetByEmail(input.Email, app.config.encryption.key)
	if err != nil {
		switch {
		// We willl use a generic error message to avoid leaking information about which
		// email addresses are registered with the system.
		case errors.Is(err, data.ErrGeneralRecordNotFound):
			v.AddError("message", "if we found a matching email address, we have sent password reset instructions to it")
			app.failedValidationResponse(w, r, v.Errors)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	// Return an error message if the user is not activated.
	if !user.Activated {
		app.inactiveAccountResponse(w, r)
		return
	}

	// ToDO: Add MFA Check Here Branching to MFATOTP Checker Handler To Verify MFA
	// Return 401 Unauthorized if the user has MFA enabled and TOTP code is not provided
	if user.MFAEnabled {
		err := app.validateTOTPResetPasswordHandler(input.TOTPCode, user)
		if err != nil {
			// if user has mfaenable and did not actually provide a TOTP code, we return an unauthorized error
			switch {
			case errors.Is(err, data.ErrInvalidTOTPCode):
				// return an unauthorized error
				app.invalidAuthenticationTokenResponse(w, r)
			default:
				// otherwise return a 500 internal server error
				app.serverErrorResponse(w, r, err)
			}
			return
		}
	}

	// Otherwise, create a new password reset token with a 45-minute expiry time.
	token, err := app.models.Tokens.New(user.ID, 45*time.Minute, data.ScopePasswordReset)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	// Email the user with their password reset token.
	app.background(func() {
		data := map[string]any{
			"passwordResetURL":   app.config.frontend.passwordreseturl + token.Plaintext,
			"passwordResetToken": token.Plaintext,
		}
		// Since email addresses MAY be case sensitive, notice that we are sending this
		// email using the address stored in our database for the user --- not to the
		// input.Email address provided by the client in this request.
		err = app.mailer.Send(user.Email, "token_password_reset.tmpl", data)
		if err != nil {
			app.logger.Error("Error sending password reset email", zap.Error(err))
		}
	})
	// Send a 202 Accepted response and confirmation message to the client.
	// But use a generic message as well
	// an email will be sent to you containing password reset instructions
	env := envelope{"message": "if we found a matching email address, we have sent password reset instructions to it"}
	err = app.writeJSON(w, http.StatusAccepted, env, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// validateTOTPResetPasswordHandler() is a helper that validates the TOTP code for the user when
// They have MFA enabled and are trying to reset their password. We check if the user has provided
// a TOTP code, if not, we cache the session and return an error. If the user has provided a TOTP code,
// We proceed and check if there is a pending MFA session, if not we return an error. If everyhing is good,
// We perform a validation on the TOTP code using data.ValidateTOTPCode, and continue to do a secret
// validation via validateAndDeleteTOTP passing the token, secret and redis key
// If everything is okay, we return nil otherwise we return an error
func (app *application) validateTOTPResetPasswordHandler(totpCode string, user *data.User) error {
	// create our redis key with RedisMFAResetPasswordPendingPrefix
	redisKey := fmt.Sprintf("%s:%d", data.RedisMFAResetPasswordPendingPrefix, user.ID)
	// check if the user has provided a TOTP code, if not, REDIS cache the session and return an error
	if totpCode == "" {
		// cache the session
		err := setToCache(context.Background(), app.RedisDB, redisKey, &data.MFASession{
			Email: user.Email,
			Value: data.MFAStatusPending,
		}, data.DefaulRedistUserMFATTLS)
		if err != nil {
			return err
		}
		return data.ErrInvalidTOTPCode
	}
	// check if there is a pending MFA session, if not we return an error
	_, err := getFromCache[*data.MFASession](context.Background(), app.RedisDB, redisKey)
	if err != nil {
		switch {
		case errors.Is(err, ErrNoDataFoundInRedis):
			// return an error
			return data.ErrRedisMFAKeyNotFound
		default:
			// return a 500 internal server error
			return err
		}
	}
	// validate the TOTP code
	mfaToken := &data.MFAToken{
		TOTPCode: totpCode,
	}
	v := validator.New()
	// validate the input
	if data.ValidateTOTPCode(v, mfaToken); !v.Valid() {
		return data.ErrInvalidTOTPCode
	}
	// verify the code and delete the secret, if there is an error, we abort
	err = app.validateAndDeleteTOTP(mfaToken.TOTPCode, user.MFASecret, redisKey)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrInvalidTOTPCode):
			return data.ErrInvalidTOTPCode
		default:
			return err
		}
	}
	// everything is okay
	return nil
}

// createManualActivationTokenHandler() is the main endpoint responsible for creating a new activation
// token for the user. This endpoint is used when the user wants to activate their account manually.
// Or when the user did not receive the activation email. We accept a users email address, validate it,
// and then check if the user exists in the database. If the user exists, we then check if the user has
// already been activated. If the user has not been activated, we create a new activation token with a 3-day
// expiry time and send it to the user's email address.
func (app *application) createManualActivationTokenHandler(w http.ResponseWriter, r *http.Request) {
	// Parse and validate the user's email address.
	var input struct {
		Email string `json:"email"`
	}
	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}
	v := validator.New()
	if data.ValidateEmail(v, input.Email); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}
	// Try to retrieve the corresponding user record for the email address. If it can't
	// be found, return an error message to the client.
	user, err := app.models.Users.GetByEmail(input.Email, app.config.encryption.key)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrGeneralRecordNotFound):
			v.AddError("email", "no matching email address found")
			app.failedValidationResponse(w, r, v.Errors)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	// Return an error if the user has already been activated.
	if user.Activated {
		v.AddError("email", "user has already been activated")
		app.failedValidationResponse(w, r, v.Errors)
		return
	}
	// Otherwise, create a new activation token.
	token, err := app.models.Tokens.New(user.ID, 3*24*time.Hour, data.ScopeActivation)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	// Email the user with their additional activation token.
	app.background(func() {
		data := map[string]any{
			"activationURL": app.config.frontend.activationurl + token.Plaintext,
			"firstName":     user.FirstName,
			"lastName":      user.LastName,
			"userID":        user.ID,
		}
		// Since email addresses MAY be case sensitive, notice that we are sending this
		// email using the address stored in our database for the user --- not to the
		// input.Email address provided by the client in this request.
		err = app.mailer.Send(user.Email, "user_welcome.tmpl", data)
		if err != nil {
			app.logger.Info("An error occurred while sending the activation email", zap.Error(err))
		}
	})
	// Send a 202 Accepted response and confirmation message to the client.
	env := envelope{"message": "an email will be sent to you containing activation instructions"}
	err = app.writeJSON(w, http.StatusAccepted, env, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// ToDo: Add handlers for recovering account with recovery codes
// 1) First, a handler that will send an email to the user with a link to recover their account
// - This handler needs to check if the user has MFA enabled, if they do, we need to send them an email
// with a link to recover their account using their recovery codes
// 2) A handler that will validate the recovery code and allow the user to reset their password
// In the process remove MFA from their account by updating the user. It also needs to set the
// recovery code to used in the database
func (app *application) initializeRecoveryByRecoveryCodes(w http.ResponseWriter, r *http.Request) {
	// Parse and validate the user's email address.
	var input struct {
		Email string `json:"email"`
	}
	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}
	v := validator.New()
	if data.ValidateEmail(v, input.Email); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}
	// Try to retrieve the corresponding user record for the email address. If it can't
	// be found, return an error message to the client.
	user, err := app.models.Users.GetByEmail(input.Email, app.config.encryption.key)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrGeneralRecordNotFound):
			v.AddError("email", "no matching email address found")
			app.failedValidationResponse(w, r, v.Errors)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	// Return an error if the user has not been activated.
	if !user.Activated {
		v.AddError("email", "user account has not been activated")
		app.failedValidationResponse(w, r, v.Errors)
		return
	}
	// Return an error if the user has not enabled MFA
	if !user.MFAEnabled {
		v.AddError("email", "user account has not enabled MFA")
		app.failedValidationResponse(w, r, v.Errors)
		return
	}
	// Otherwise, create a new recovery token with 15-minute expiry time.
	token, err := app.models.Tokens.New(user.ID, 15*time.Minute, data.ScopeRecovery)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	// Email the user with their additional recovery token.
	app.background(func() {
		data := map[string]any{
			"recoveryCodesURL": app.config.frontend.recoveryurl,
			"tokenPlaintext":   token.Plaintext,
			"firstName":        user.FirstName,
			"lastName":         user.LastName,
		}
		// Since email addresses MAY be case sensitive, notice that we are sending this
		// email using the address stored in our database for the user --- not to the
		// input.Email address provided by the client in this request.
		err = app.mailer.Send(user.Email, "account_recovery.tmpl", data)
		if err != nil {
			app.logger.Info("An error occurred while sending the recovery email", zap.Error(err))
		}
	})
	// Send a 202 Accepted response and confirmation message to the client.
	env := envelope{"message": "an email will be sent to you containing recovery instructions"}
	err = app.writeJSON(w, http.StatusAccepted, env, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}

}

// validateRecoveryCodeHandler() is a handler method that verifies the recovery code for the user when
// They have MFA enabled and are trying to recover their account. This endpoint will receive the []recovery code
// and the token from the email sent via the initializeRecoveryByRecoveryCodes() handler. We first validate the
// []codes as well as the tokenplaintext, Then we use GetForToken to get the recovery code from the database passing
// in data.ScopeRecovery as the scope. We then need to get the hashed version of the recovery code from the database
// and compare it to the hashed version of the concatenated recovery codes. If they match, we proceed to update the user
// and remove the MFA from their account. We also need to set the recovery code to used in the database
func (app *application) validateRecoveryCodeHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		RecoveryCodes  []string `json:"recovery_codes"`
		TokenPlaintext string   `json:"token_plaintext"`
	}
	// read the body into the input struct
	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}
	// validate the input
	v := validator.New()
	// validate the input
	if data.ValidateRecovery(v, input.RecoveryCodes, input.TokenPlaintext); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}
	// get the user from the database
	user, err := app.models.Users.GetForToken(data.ScopeRecovery, input.TokenPlaintext, app.config.encryption.key)
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
	// get the recovery code from the database
	recoveryCode, err := app.models.MFAManager.GetRecoveryCodesByUserID(user.ID)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	//concatenate the recovery codes
	joinedRecoveryCodes := strings.Join(input.RecoveryCodes, "")
	// we will use the matches method in  recoveryCodes to compare the hashed version of the concatenated recovery codes
	// passing in the hashedRecoveryCode
	// check if the password matches
	match, err := recoveryCode.Matches(joinedRecoveryCodes)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	// compare it to the hashed version of the concatenated recovery codes
	// if password doesn't match then we shout
	if !match {
		app.invalidCredentialsResponse(w, r)
		return
	}
	// update the user and remove the MFA from their account
	user.MFAEnabled = false
	user.MFASecret = ""
	// Save the user to the DB
	err = app.models.Users.UpdateUser(user, app.config.encryption.key)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	// set the recovery code to used in the database
	err = app.models.MFAManager.MarkRecoveryCodeAsUsed(recoveryCode.ID, user.ID)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	// log
	app.logger.Info("Recovery validation attempt", zap.Int64("user_id", user.ID), zap.Bool("success", match))
	// ToDo: Think if we should delete the recovery code from the database

	// Email the user using account_recovery_acknowledgment.tmpl acknowledging that their account has been recovered
	// and that they can now proceed to reset their password
	app.background(func() {
		data := map[string]any{
			"firstName":        user.FirstName,
			"lastName":         user.LastName,
			"resetPasswordURL": app.config.frontend.passwordreseturl,
		}
		err := app.mailer.Send(user.Email, "account_recovery_acknowledgment.tmpl", data)
		if err != nil {
			app.logger.Error("Error sending recovery acknowledgment email", zap.Error(err))
		}
	})
	// Send a 200 OK response and confirmation message to the client.
	message := envelope{"message": "Your account has been successfully recovered"}
	err = app.writeJSON(w, http.StatusOK, message, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
	// send notification to the user
	notificationContent := data.NotificationContent{
		Message: fmt.Sprintf("%s, your account has been successfully recovered. We recommend you re-enable MFA and link to your current device", user.FirstName),
		Meta: data.NotificationMeta{
			Url:      app.config.frontend.profileurl,
			ImageUrl: app.config.frontend.applogourl,
			Tags:     "recovery,mfa,security",
		},
	}
	err = app.PublishNotificationToRedis(user.ID, data.NotificationTypeAccount, notificationContent)
	if err != nil {
		app.logger.Error("Error publishing MFA notification to redis", zap.Error(err))
	}
}
