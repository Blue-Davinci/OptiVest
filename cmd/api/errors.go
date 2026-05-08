package main

import (
	"errors"
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

// badRequestResponse sends a 400 Bad Request envelope to the client - or, if
// err wraps ErrUnsupportedMediaType, transparently delegates to
// unsupportedMediaTypeResponse so the wire status is the more specific 415.
//
// The auto-routing exists because every handler in this codebase follows
// the same pattern after readJSON failures:
//
//	err := app.readJSON(w, r, &input)
//	if err != nil {
//	    app.badRequestResponse(w, r, err)
//	    return
//	}
//
// Without this routing, picking up the new Content-Type validation would
// require touching ~50 call sites across cmd/api/*.go to do the
// errors.Is dispatch by hand. Centralizing it here keeps the existing
// handlers untouched and means any future readJSON-wrapped sentinels
// (ErrPayloadTooLarge, ErrInvalidJSON, etc.) can be routed the same way.
func (app *application) badRequestResponse(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, ErrUnsupportedMediaType) {
		app.unsupportedMediaTypeResponse(w, r)
		return
	}
	app.errorResponse(w, r, http.StatusBadRequest, err.Error())
}

// unsupportedMediaTypeResponse returns 415 with a constant client-facing
// message. We deliberately do NOT echo the rejected Content-Type back to
// the client: the value is something the client already knows (they
// sent it), and surfacing it in the error envelope makes future log
// pipelines harder to redact if the field ever turns out to carry a
// secret-like token. The full diagnostic context (which media type was
// rejected, why mime.ParseMediaType failed) is preserved on the
// server-side error chain via fmt.Errorf("%w: ...") in
// validateJSONContentType - logRequests captures it for ops.
func (app *application) unsupportedMediaTypeResponse(w http.ResponseWriter, r *http.Request) {
	app.errorResponse(w, r, http.StatusUnsupportedMediaType, "Content-Type must be application/json")
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
