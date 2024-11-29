package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/Blue-Davinci/OptiVest/internal/data"
	"github.com/Blue-Davinci/OptiVest/internal/validator"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"go.uber.org/zap"
)

// setupMFAHandler() sets up 2FA for a user. We generate a new Mfa secret key
// encrypt it, and save it into the DB. We then generate a QR code for the user
func (app *application) setupMFAHandler(w http.ResponseWriter, r *http.Request) {
	// Get the user from the context
	user := app.contextGetUser(r)
	// redis key
	redisKey := fmt.Sprintf("%s:%d", data.RedisMFASetupPendingPrefix, user.ID)
	// Check if the user has already enabled MFA
	if user.MFAEnabled {
		app.badRequestResponse(w, r, fmt.Errorf("MFA is already enabled for this user"))
		return
	}
	// check if there is an existing pending session in REDIS, we do this by checking if the key exists
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
	// Generate a new MFA secret
	secret, err := app.totpTokenGenerator(user.Email, redisKey, data.MFAStatusPending)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	// Save the secret to the user
	user.MFASecret = secret.Secret()
	// if succesful, update the user's key// in the DB
	err = app.models.Users.UpdateUser(user, app.config.encryption.key)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	app.logger.Info("MFA setup pending status saved to redis", zap.String("used key", redisKey))

	// Return the QR code to the user
	err = app.writeJSON(w, http.StatusOK, envelope{"qr_code": secret.URL()}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// totpTokenGenerator() generates a new TOTP token for a user. We generate a new
// secret key for the user, and save the user's email and the status of the 2FA setup session to redis
func (app *application) totpTokenGenerator(userEmail, redisKey, value string) (*otp.Key, error) {
	secret, err := totp.Generate(totp.GenerateOpts{
		Issuer:      app.config.api.name,
		AccountName: userEmail,
	})
	app.logger.Info("Email used", zap.String("email", userEmail), zap.String("Issuer", app.config.api.name))
	if err != nil {
		return nil, err
	}
	// save a 2fa status to redis including their user id
	err = setToCache(context.Background(), app.RedisDB, redisKey, &data.MFASession{
		Email: userEmail,
		Value: value,
	}, data.DefaulRedistUserMFATTLS)
	if err != nil {
		return nil, err
	}
	app.logger.Info("-- MFA setup pending status saved to redis", zap.String("key", redisKey))
	return secret, nil
}

// verifiy2FASetupHandler() verifies the 2FA setup for a user. We check if the
// user has a pending 2FA setup in redis, and if they do, we proceed to get the code
// that they send, otherwise we return an error. We verify the TOTP code they sent
// if it's correct, we save the user's MFA status to the DB, and remove the pending
// status from redis
func (app *application) verifiy2FASetupHandler(w http.ResponseWriter, r *http.Request) {
	// prepare input struct to obtain the sent back code
	var input struct {
		TOTPCode string `json:"totp_code"`
	}
	// Get the user from the context
	user := app.contextGetUser(r)
	// Check if the user has already enabled MFA
	if user.MFAEnabled {
		app.badRequestResponse(w, r, data.ErrMFANotEnabled)
		return
	}
	// Check if the user has a pending 2FA setup
	// If they do NOT have a pending 2FA setup, we return an error
	redisKey := fmt.Sprintf("%s:%d", data.RedisMFASetupPendingPrefix, user.ID)
	mfaSession, err := getFromCache[*data.MFASession](context.Background(), app.RedisDB, redisKey)
	if err != nil {
		switch {
		case errors.Is(err, ErrNoDataFoundInRedis):
			app.badRequestResponse(w, r, data.ErrRedisMFAKeyNotFound)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	// verify the user's email is the same as the one in the session
	if (*mfaSession).Email != user.Email {
		app.badRequestResponse(w, r, fmt.Errorf("there is an issue with your MFA session. Please try again"))
		return
	}
	// read the body into the input struct
	err = app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}
	// make an mfatoken
	mfaToken := &data.MFAToken{
		TOTPCode: input.TOTPCode,
	}
	v := validator.New()
	// validate the input
	if data.ValidateTOTPCode(v, mfaToken); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}
	// log the value
	app.logger.Info("MFA setup pending status fetched from redis", zap.String("key", redisKey))
	// Verify the code and delete the secret, if there is an error, we abort
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
	// before we update the user, let us attampt to save & generate the recovery codes
	// this is to ensure that the user has recovery codes in case they lose their device
	// and if it fails, we do not update the user
	recoveryCodes, err := app.models.MFAManager.CreateNewRecoveryCode(user.ID)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	// Update the user's MFA status
	user.MFAEnabled = true
	// Save the user to the DB
	err = app.models.Users.UpdateUser(user, app.config.encryption.key)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	app.logger.Info("MFA setup pending status removed from redis", zap.String("key", redisKey))
	// Return a success response and the recovery codes
	err = app.writeJSON(w, http.StatusOK, envelope{
		"first_name":       user.FirstName,
		"last_name":        user.LastName,
		"message":          "Your MFA request has been succesfully enabled. Please save your recovery codes",
		"recovery_details": recoveryCodes,
	}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
	// send an email to the user
	app.background(func() {
		data := map[string]any{
			"firstName": user.FirstName,
			"lastName":  user.LastName,
		}
		err := app.mailer.Send(user.Email, "mfa_acknowledgment.tmpl", data)
		if err != nil {
			app.logger.Error("Error sending 2fa acknowledgment email", zap.Error(err))
		}
	})
	// send a notification to the user
	notificationContent := data.NotificationContent{
		Message: fmt.Sprintf(("%s, your MFA request has been succesfully enabled. Remember to save your recovery codes safely and securely. They are the only way to get your account in cases where your device is lost or damaged"), user.FirstName),
		Meta: data.NotificationMeta{
			Url:      app.config.frontend.profileurl,
			ImageUrl: app.config.frontend.applogourl,
			Tags:     "mfa,security",
		},
	}
	err = app.PublishNotificationToRedis(user.ID, data.NotificationTypeAccount, notificationContent)
	if err != nil {
		app.logger.Error("Error publishing MFA notification to redis", zap.Error(err))
	}
}

// validateAndDeleteTOTP() validates the TOTP code that the user sends. If the code is
// valid, we delete the secret from the DB
func (app *application) validateAndDeleteTOTP(TOTPCode, MFASecret, redisKey string) error {
	opts := totp.ValidateOpts{
		Period:    30,                // Time step in seconds (default is 30)
		Skew:      1,                 // Allowable time skew in steps (default is 1)
		Digits:    otp.DigitsSix,     // Number of digits in the TOTP code (default is 6)
		Algorithm: otp.AlgorithmSHA1, // Hashing algorithm (default is SHA1)
	}
	// Validate the TOTP code with custom options
	valid, err := totp.ValidateCustom(TOTPCode, MFASecret, time.Now(), opts)
	if err != nil {
		return err
	}
	app.logger.Info("TOTP code validation", zap.String("code", TOTPCode), zap.String("secret", MFASecret), zap.Bool("valid", valid))
	if !valid {
		return data.ErrInvalidTOTPCode
	}
	// if the code is valid, we delete the secret from REDIS
	delCmd := app.RedisDB.Del(context.Background(), redisKey)
	if err := delCmd.Err(); err != nil {
		return err
	}

	return nil
}
