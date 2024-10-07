package main

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// error response structure for WebSocket clients.
type wsErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

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

// The inactiveAccountResponse() method will return 401 inactive account error, that is the account
// needs to be activated to proceed.
func (app *application) inactiveAccountResponse(w http.ResponseWriter, r *http.Request) {
	message := "your user account must be activated to access this resource"
	app.errorResponse(w, r, http.StatusForbidden, message)
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

// The notFoundResponse() method will be used to send a 404 Not Found status code and
// JSON response to the client.
func (app *application) notFoundResponse(w http.ResponseWriter, r *http.Request) {
	message := "the requested resource could not be found"
	app.errorResponse(w, r, http.StatusNotFound, message)
}

// wsErrorResponse() is a WebSocket method that sends websocket error response.
func (app *application) wsErrorResponse(conn *websocket.Conn, errorCode int, errorMessage string) {
	// Create a WebSocket error message in JSON format.
	errorResponse := wsErrorResponse{
		Error:   http.StatusText(errorCode),
		Message: errorMessage,
	}

	// Marshal the error response into JSON.
	response, err := json.Marshal(errorResponse)
	if err != nil {
		// If JSON marshalling fails, log the error and close the connection.
		app.logger.Error("Failed to marshal error response", zap.Error(err))
		conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseInternalServerErr, "Internal Server Error"))
		conn.Close()
		return
	}

	// Send the error response to the WebSocket client.
	err = conn.WriteMessage(websocket.TextMessage, response)
	if err != nil {
		// Log and close the connection if writing fails.
		app.logger.Error("Failed to send error response", zap.Error(err))
		conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseInternalServerErr, "Internal Server Error"))
		conn.Close()
	}
}

// wsInvalidAuthenticationResponse() sends an authentication error for WebSocket connection.
func (app *application) wsInvalidAuthenticationResponse(conn *websocket.Conn) {
	app.wsErrorResponse(conn, http.StatusUnauthorized, "Invalid authentication credential")
	conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "Unauthorized"))
	conn.Close()
}

// wsWebSocketUpgradeError() sends an error response when upgrading to WebSocket fails.
func (app *application) wsWebSocketUpgradeError(conn *websocket.Conn) {
	app.wsErrorResponse(conn, http.StatusInternalServerError, "Failed to upgrade to WebSocket")
	conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseInternalServerErr, "upgrade error"))
	conn.Close()
}

// wsServerErrorRespomse() sends a server error response for WebSocket connection.
func (app *application) wsServerErrorResponse(conn *websocket.Conn, message string) {
	app.wsErrorResponse(conn, http.StatusInternalServerError, message)
	conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseInternalServerErr, "Internal Server Error"))
	conn.Close()
}

// wsMaximumConnectionsResponse() sends a maximum connections error for WebSocket connection.
func (app *application) wsMaxConnectionsResponse(conn *websocket.Conn) {
	app.wsErrorResponse(conn, http.StatusServiceUnavailable, "Maximum connections reached")
	conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseTryAgainLater, "Service Unavailable"))
	conn.Close()
}
