package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/Blue-Davinci/OptiVest/internal/data"
	"github.com/Blue-Davinci/OptiVest/internal/validator"
	"github.com/go-redis/redis/v8"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"go.uber.org/zap"
)

// setupMFAHandler() sets up 2FA for a user. We generate a new Mfa secret key
// encrypt it, and save it into the DB. We then generate a QR code for the user
func (app *application) setupMFAHandler(w http.ResponseWriter, r *http.Request) {
	// Get the user from the context
	user := app.contextGetUser(r)
	// Check if the user has already enabled MFA
	if user.MFAEnabled {
		app.badRequestResponse(w, r, fmt.Errorf("MFA is already enabled for this user"))
		return
	}
	// check if there is an existing pending session
	saveValue, _, err := app.fetchRedisDataForUser(data.RedisMFASetupPendingPrefix, user.ID)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRedisMFAKeyNotFound):
			// if we could not find the key, we continue
			// continue
		default:
			app.serverErrorResponse(w, r, err)
			return
		}
	}
	// if the key exists, we return an error
	if saveValue == "pending" {
		app.badRequestResponse(w, r, data.ErrRedisMFAKeyAlreadyExists)
		return
	}
	// Generate a new MFA secret
	secret, redisKey, err := app.totpTokenGenerator(user.Email, data.RedisMFASetupPendingPrefix, "pending", user.ID)
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

	app.logger.Info("MFA setup pending status saved to redis", zap.String("used key", redisKey), zap.String("saved value", saveValue))

	// Return the QR code to the user
	err = app.writeJSON(w, http.StatusOK, envelope{"qr_code": secret.URL()}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// fetchRedisDataForUser() fetches the data from redis for a user. We use this
// function to fetch the MFA setup status for a user
func (app *application) fetchRedisDataForUser(key string, userID int64) (string, string, error) {
	// Fetch the data from redis
	redisKey := app.returnFormattedRedisKeys(key, userID)
	statusCmd := app.RedisDB.Get(context.Background(), redisKey)
	value, err := statusCmd.Result()
	app.logger.Info("MFA status fetched from redis", zap.String("key", redisKey), zap.String("value", value))
	if err == redis.Nil {
		return "", "", data.ErrRedisMFAKeyNotFound
	} else if err != nil {
		return "", "", err
	}
	return value, redisKey, nil
}

// TTL
func (app *application) totpTokenGenerator(userEmail, prefix, value string, userID int64) (*otp.Key, string, error) {
	secret, err := totp.Generate(totp.GenerateOpts{
		Issuer:      app.config.api.name,
		AccountName: userEmail,
	})
	app.logger.Info("Email used", zap.String("email", userEmail), zap.String("Issuer", app.config.api.name))
	if err != nil {
		return nil, "", err
	}
	// save a 2fa status to redis including their user id
	redisKey := app.returnFormattedRedisKeys(prefix, userID)
	statusCmd := app.RedisDB.SetEX(context.Background(), redisKey, value, data.DefaulRedistUserMFATTLS)
	if err := statusCmd.Err(); err != nil {
		return nil, "", err
	}
	app.logger.Info("-- MFA setup pending status saved to redis", zap.String("key", redisKey))
	app.fetchRedisDataForUser(prefix, userID)
	return secret, redisKey, nil
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
	value, redisKey, err := app.fetchRedisDataForUser(data.RedisMFASetupPendingPrefix, user.ID)
	if err != nil {
		app.serverErrorResponse(w, r, err)
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
	app.logger.Info("MFA setup pending status fetched from redis", zap.String("key", redisKey), zap.String("value", value))
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
	// Update the user's MFA status
	user.MFAEnabled = true
	// Save the user to the DB
	err = app.models.Users.UpdateUser(user, app.config.encryption.key)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	app.logger.Info("MFA setup pending status removed from redis", zap.String("key", redisKey))

	// Return a success response
	err = app.writeJSON(w, http.StatusOK, envelope{
		"user_id":    user.ID,
		"first_name": user.FirstName,
		"last_name":  user.LastName,
		"message":    "MFA Succesfully enabled",
	}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

func (app *application) validateAndDeleteTOTP(TOTPCode, MFASecret, redisKey string) error {
	// validate our TOTP code
	valid := totp.Validate(TOTPCode, MFASecret)
	app.logger.Info("TOTP code validation", zap.String("code", TOTPCode), zap.String("secret", MFASecret), zap.Bool("valid", valid))
	if !valid {
		return data.ErrInvalidTOTPCode
	}
	// if the code is valid, we delete the secret
	delCmd := app.RedisDB.Del(context.Background(), redisKey)
	if err := delCmd.Err(); err != nil {
		return err
	}

	return nil
}
