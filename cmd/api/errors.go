package main

import (
	"net/http"

	"go.uber.org/zap"
)

func (app *application) logError(r *http.Request, err error) {
	// Use the PrintError() method to log the error message, and include the current
	// request method and URL as properties in the log entry.
	app.logger.Error(err.Error(), zap.String("request_method", r.Method), zap.String("request_url", r.URL.String()))

}

// The errorResponse() method is a generic helper for sending JSON-formatted error
// messages to the client with a given status code.
func (app *application) errorResponse(w http.ResponseWriter, r *http.Request, status int, message any) {
	env := envelope{"error": message}
	// Write the response using the writeJSON() helper. If this happens to return an
	// error then log it, and fall back to sending the client an empty response with a
	// 500 Internal Server Error status code.
	err := app.writeJSON(w, status, env, nil)
	if err != nil {
		app.logError(r, err)
		w.WriteHeader(500)
	}
}

// The serverErrorResponse() method will be used when our application encounters an
// unexpected problem at runtime. It logs the detailed error message, then uses the
// errorResponse() helper to send a 500 Internal Server Error status code and JSON
// response (containing a generic error message) to the client.
func (app *application) serverErrorResponse(w http.ResponseWriter, r *http.Request, err error) {
	app.logError(r, err)
	message := "the server encountered a problem and could not process your request"
	app.errorResponse(w, r, http.StatusInternalServerError, message)
}

// The invalidAuthenticationTokenResponse() method will return invalid token error
func (app *application) invalidAuthenticationTokenResponse(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("WWW-Authenticate", "Bearer")
	message := "invalid or missing authentication token"
	app.errorResponse(w, r, http.StatusUnauthorized, message)
}

// The authenticationRequiredResponse() method will return 403 authentication required error, that
// is the client needs to register + auth their account to proceed.
func (app *application) authenticationRequiredResponse(w http.ResponseWriter, r *http.Request) {
	message := "you must be authenticated to access this resource"
	app.errorResponse(w, r, http.StatusUnauthorized, message)
}

// The inactiveAccountResponse() method returns 423 Locked. It is used on
// AUTHENTICATED routes (`requireActivatedUser` middleware) where the caller
// has already proven possession of a valid bearer token and the server is
// telling them their account is in a locked state - that response leaks
// nothing new because the caller is already known. Do NOT use it on
// unauthenticated routes (login, password-reset, recovery, activation):
// those leak account-state to anonymous probes and must use
// accountRecoveryEligibleResponse instead.
func (app *application) inactiveAccountResponse(w http.ResponseWriter, r *http.Request) {
	message := "your user account must be activated to access this resource"
	app.errorResponse(w, r, http.StatusLocked, message)
}

// accountRecoveryEligibleResponse returns the canonical 202 Accepted envelope
// used by every unauthenticated account-recovery, activation, and password-
// reset flow. The wire format is byte-identical regardless of which branch
// the caller actually took (email-not-found, already-activated, MFA-not-
// enabled, or the genuine happy path), so the response itself reveals
// nothing about the requester's account state. Callers MUST log the
// real branch at WARN with the request_id (and user_id where available)
// so support and incident-response can still triage legitimate users.
//
// Status code is 202 specifically because none of these flows synchronously
// confirm the email was sent - delivery happens via app.background. 202
// matches that semantics ("we'll get to it") and avoids the 200-vs-202
// distinction being itself a tell.
func (app *application) accountRecoveryEligibleResponse(w http.ResponseWriter, r *http.Request) {
	env := envelope{"message": "if your account is eligible for this action, an email will be sent with further instructions"}
	if err := app.writeJSON(w, http.StatusAccepted, env, nil); err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// The badRequestResponse() method will be used to send a 400 Bad Request status code and
// JSON response to the client.
func (app *application) badRequestResponse(w http.ResponseWriter, r *http.Request, err error) {
	app.errorResponse(w, r, http.StatusBadRequest, err.Error())
}

// Note that the errors parameter here has the type map[string]string, which is exactly
// the same as the errors map contained in our Validator type.
func (app *application) failedValidationResponse(w http.ResponseWriter, r *http.Request, errors map[string]string) {
	app.errorResponse(w, r, http.StatusUnprocessableEntity, errors)
}

// The rateLimitExceededResponse() method will return a 429 too many requests error.
func (app *application) rateLimitExceededResponse(w http.ResponseWriter, r *http.Request) {
	message := "rate limit exceeded"
	app.errorResponse(w, r, http.StatusTooManyRequests, message)
}

// The editConflictResponse() method will be used to send a 409 Conflict status code and
// JSON response to the client.
func (app *application) editConflictResponse(w http.ResponseWriter, r *http.Request) {
	message := "unable to update the record due to an edit conflict, please try again"
	app.errorResponse(w, r, http.StatusConflict, message)
}

// The invalidCredentialsResponse() method will return invalid token credential error
func (app *application) invalidCredentialsResponse(w http.ResponseWriter, r *http.Request) {
	message := "invalid authentication credentials"
	app.errorResponse(w, r, http.StatusUnauthorized, message)
}

func (app *application) sessionExpiredResponse(w http.ResponseWriter, r *http.Request) {
	message := "session expired or not found. please log in again"
	app.errorResponse(w, r, http.StatusUnauthorized, message)
}

// The notFoundResponse() method will be used to send a 404 Not Found status code and
// JSON response to the client.
func (app *application) notFoundResponse(w http.ResponseWriter, r *http.Request) {
	message := "the requested resource could not be found"
	app.errorResponse(w, r, http.StatusNotFound, message)
}
