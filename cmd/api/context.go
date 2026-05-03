package main

import (
	"context"
	"net/http"

	"github.com/Blue-Davinci/OptiVest/internal/data"
)

// Define a custom contextKey type, with the underlying type string.
type contextKey string

// Convert the string "user" to a contextKey type and assign it to the userContextKey
// constant. We'll use this constant as the key for getting and setting user information
// in the request context.
const (
	userContextKey       = contextKey("user")
	connIDContextKey     = contextKey("conn_id")
	requestIDContextKey  = contextKey("req_id")
	requestLogContextKey = contextKey("req_log")
)

// contextConnID returns the per-connection ID stamped by ConnContext on
// http.Server, or 0 if the context did not flow through one of our servers
// (e.g. a synthetic request constructed in tests). Useful for log correlation.
func contextConnID(ctx context.Context) int64 {
	id, _ := ctx.Value(connIDContextKey).(int64)
	return id
}

// contextRequestID returns the per-request ID set by the requestID
// middleware. Returns "" if the context did not flow through that middleware
// (e.g. a synthetic request constructed in tests).
func contextRequestID(ctx context.Context) string {
	s, _ := ctx.Value(requestIDContextKey).(string)
	return s
}

// requestLog is a mutable, request-scoped record that the logRequests
// middleware uses to assemble its single per-request log line. Middlewares
// deeper in the chain (notably authenticate) can write fields into this
// struct via contextRequestLog; the logger reads them after next.ServeHTTP
// returns.
//
// I picked a shared mutable holder over the more idiomatic "wrap r with a
// new context per write" approach because the context update would not
// propagate back up to the outer logging middleware (each WithValue produces
// a new request struct visible only to inner handlers). A single pointer in
// the original context lets every layer collaborate without restructuring
// the request.
type requestLog struct {
	userID int64 // 0 means unauthenticated / anonymous
}

// contextRequestLog returns the per-request log holder set by the
// logRequests middleware, or nil if the context did not flow through it
// (e.g. a synthetic request in tests). Callers must always nil-check.
func contextRequestLog(ctx context.Context) *requestLog {
	rl, _ := ctx.Value(requestLogContextKey).(*requestLog)
	return rl
}

// The contextSetUser() method returns a new copy of the request with the provided
// User struct added to the context. Note that we use our userContextKey constant as the
// key.
func (app *application) contextSetUser(r *http.Request, user *data.User) *http.Request {
	ctx := context.WithValue(r.Context(), userContextKey, user)
	//app.log.PrintInfo("set user in request context", map[string]string{"name": user.Name, "email": user.Email})
	return r.WithContext(ctx)
}

// The contextGetUser() retrieves the User struct from the request context. The only
// time that we'll use this helper is when we logically expect there to be User struct
// value in the context, and if it doesn't exist it will firmly be an 'unexpected' error.
// As we discussed earlier in the book, it's OK to panic in those circumstances.
func (app *application) contextGetUser(r *http.Request) *data.User {
	user, ok := r.Context().Value(userContextKey).(*data.User)
	if !ok {
		panic("missing user value in request context")
	}
	return user
}
