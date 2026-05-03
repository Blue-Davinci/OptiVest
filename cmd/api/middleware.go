package main

import (
	"context"
	"errors"
	"expvar"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/Blue-Davinci/OptiVest/internal/data"
	"github.com/felixge/httpsnoop"
	"github.com/go-redis/redis_rate/v10"
	"github.com/tomasen/realip"
	"go.uber.org/zap"
)

func (app *application) recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Create a deferred function (which will always be run in the event of a panic
		// as Go unwinds the stack).
		defer func() {
			// Use the builtin recover function to check if there has been a panic or
			// not.
			if err := recover(); err != nil {
				// If there was a panic, set a "Connection: close" header on the
				// response. This acts as a trigger to make Go's HTTP server
				// automatically close the current connection after a response has been
				// sent.
				w.Header().Set("Connection", "close")
				// The value returned by recover() has the type any, so we use
				// fmt.Errorf() to normalize it into an error and call our
				// serverErrorResponse() helper. In turn, this will log the error using
				// our custom Logger type at the ERROR level and send the client a 500
				// Internal Server Error response.
				app.serverErrorResponse(w, r, fmt.Errorf("%s", err))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func (app *application) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Add the "Vary: Authorization" header to the response. This indicates to any
		// caches that the response may vary based on the value of the Authorization
		// header in the request.
		w.Header().Add("Vary", "Authorization")
		// Retrieve the value of the Authorization header from the request. This will
		// return the empty string "" if there is no such header found.
		user, err := app.aunthenticatorHelper(r)
		if user == data.AnonymousUser {
			r = app.contextSetUser(r, data.AnonymousUser)
			next.ServeHTTP(w, r)
			return
		}
		if err != nil {
			switch {
			case errors.Is(err, ErrInvalidAuthentication):
				app.invalidAuthenticationTokenResponse(w, r)
			case errors.Is(err, data.ErrGeneralRecordNotFound):
				app.invalidAuthenticationTokenResponse(w, r)
			default:
				app.serverErrorResponse(w, r, err)
			}
			return
		}
		// Call the contextSetUser() helper to add the user information to the request
		// context.
		r = app.contextSetUser(r, user)
		// Call the next handler in the chain.
		next.ServeHTTP(w, r)
	})
}

// Create a new requireAuthenticatedUser() middleware to check that a user is not
// anonymous.
func (app *application) requireAuthenticatedUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Use the contextGetUser() helper to retrieve the user
		// information from the request context.
		user := app.contextGetUser(r)
		// If the user is anonymous, then call the authenticationRequiredResponse() to
		// inform the client that they should authenticate before trying again.
		if user.IsAnonymous() {
			app.authenticationRequiredResponse(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// Checks that a user is both authenticated and activated.
func (app *application) requireActivatedUser(next http.Handler) http.Handler {
	// Rather than returning this http.HandlerFunc we assign it to the variable fn.
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := app.contextGetUser(r)
		// If the user is not activated, use the inactiveAccountResponse() helper to
		// inform them that they need to activate their account.
		if !user.Activated {
			app.inactiveAccountResponse(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// rateLimitRedisKeyPrefix namespaces every rate-limit key in Redis so we
// don't collide with other Redis users (cache, MFA sessions, notifications).
const rateLimitRedisKeyPrefix = "optivest:ratelimit:ip:"

// rateLimitRedisTimeout caps how long we'll wait on Redis for a single
// limiter check. Keep it tight: every request blocks on this. If Redis is
// unhealthy we want to fail-open quickly rather than degrade tail latency.
const rateLimitRedisTimeout = 200 * time.Millisecond

// Rate-limiter expvar metrics, exposed on /debug/vars. Counters use Float
// because expvar.Int.Add takes int64 and we want fractional Δ atomically;
// in practice we only Add(1), but Float is the standard pattern for "this
// is a counter that ops will graph as a rate".
var (
	rateLimitAllows     = expvar.NewInt("rate_limiter_allowed_total")
	rateLimitDenies     = expvar.NewInt("rate_limiter_denied_total")
	rateLimitRedisErrs  = expvar.NewInt("rate_limiter_redis_errors_total")
	rateLimitFailOpens  = expvar.NewInt("rate_limiter_fail_open_total")
	rateLimitDisabled   = expvar.NewInt("rate_limiter_disabled_total")
	rateLimitConfigured = expvar.NewString("rate_limiter_configured")
)

// rateLimit returns a middleware that enforces a per-IP request rate using a
// Redis-backed GCRA bucket (github.com/go-redis/redis_rate/v10). Replaces the
// previous in-memory token-bucket implementation, which under-counted by a
// factor of N when running N application instances behind a load balancer:
// each instance had its own clients map, so the configured limit was the
// per-pod limit, not the per-cluster limit.
//
// Behaviour
//
//   - Keying is per real-IP (via tomasen/realip, same as before). User-keying
//     is intentionally out of scope for this PR because rateLimit runs before
//     authenticate in the global middleware chain — see routes.go.
//   - On allow, sets standard rate-limit response headers so well-behaved
//     clients can self-throttle:
//       X-RateLimit-Limit       <burst>
//       X-RateLimit-Remaining   <tokens left in window>
//   - On deny, also sets Retry-After (seconds, integer per RFC 7231) and
//     X-RateLimit-Reset (seconds until the bucket refills) so clients have
//     two equivalent ways to read the back-off.
//   - On Redis error, FAILS OPEN: the request is allowed through and a
//     warning is logged + a metric incremented. Rationale: the cost of
//     letting traffic through during a Redis outage is bounded (we still
//     have upstream LB, downstream DB pool limits, etc.), but the cost of
//     fail-closed is total API outage every time Redis blips. Ops should
//     alert on rate_limiter_redis_errors_total > 0 so this never goes
//     unnoticed.
//   - Honours app.config.limiter.enabled: when false, every request is
//     allowed without touching Redis (counted via rate_limiter_disabled_total).
func (app *application) rateLimit(next http.Handler) http.Handler {
	// Build the Limit once at middleware-construction time. redis_rate.Limit
	// requires an integer Rate per Period; the pre-existing config flag is a
	// float64. We round up to the nearest integer to err on the side of
	// being slightly more permissive than the operator asked for, and warn
	// loudly if precision was lost so the misconfiguration is visible.
	rate := int(math.Ceil(app.config.limiter.rps))
	if rate < 1 {
		rate = 1
	}
	if float64(rate) != app.config.limiter.rps {
		app.logger.Warn("limiter-rps was rounded up to the nearest integer; sub-1-rps configurations are not supported by the redis_rate v10 limit type",
			zap.Float64("configured_rps", app.config.limiter.rps),
			zap.Int("effective_rps", rate),
		)
	}
	limit := redis_rate.Limit{
		Rate:   rate,
		Burst:  app.config.limiter.burst,
		Period: time.Second,
	}
	rateLimitConfigured.Set(fmt.Sprintf("rps=%d burst=%d enabled=%t", rate, limit.Burst, app.config.limiter.enabled))

	limiter := redis_rate.NewLimiter(app.RedisDB)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !app.config.limiter.enabled {
			rateLimitDisabled.Add(1)
			next.ServeHTTP(w, r)
			return
		}

		ip := realip.FromRequest(r)
		key := rateLimitRedisKeyPrefix + ip

		// Bound the Redis call independently of the request's context so a
		// slow Redis can't push a request's tail latency past
		// rateLimitRedisTimeout. Still inherits cancellation from the
		// request — if the client disconnects, we abort the limiter check
		// too.
		ctx, cancel := context.WithTimeout(r.Context(), rateLimitRedisTimeout)
		defer cancel()

		res, err := limiter.Allow(ctx, key, limit)
		if err != nil {
			// Fail-open. Log loudly so this is observable; ops should alert
			// on rate_limiter_redis_errors_total > 0. Log at WARN, not
			// ERROR, because a single transient Redis error shouldn't page —
			// sustained errors should.
			rateLimitRedisErrs.Add(1)
			rateLimitFailOpens.Add(1)
			app.logger.Warn("rate limiter Redis error; failing open",
				zap.String("ip", ip),
				zap.Error(err),
			)
			next.ServeHTTP(w, r)
			return
		}

		// Inform clients about their budget on every response so well-behaved
		// callers can self-throttle without ever hitting 429.
		w.Header().Set("X-RateLimit-Limit", strconv.Itoa(limit.Burst))
		w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(res.Remaining))

		if res.Allowed > 0 {
			rateLimitAllows.Add(1)
			next.ServeHTTP(w, r)
			return
		}

		rateLimitDenies.Add(1)
		// Retry-After per RFC 7231 §7.1.3 is "delta-seconds" (integer); round
		// up so we never tell the client to retry sooner than the bucket
		// actually refills.
		retry := int(math.Ceil(res.RetryAfter.Seconds()))
		if retry < 1 {
			retry = 1
		}
		w.Header().Set("Retry-After", strconv.Itoa(retry))
		w.Header().Set("X-RateLimit-Reset", strconv.Itoa(int(math.Ceil(res.ResetAfter.Seconds()))))
		app.rateLimitExceededResponse(w, r)
	})
}
func (app *application) metrics(next http.Handler) http.Handler {
	// Initialize the new expvar variables when the middleware chain is first built.
	totalRequestsReceived := expvar.NewInt("total_requests_received")
	totalResponsesSent := expvar.NewInt("total_responses_sent")
	totalProcessingTimeMicroseconds := expvar.NewInt("total_processing_time_μs")
	totalResponsesSentByStatus := expvar.NewMap("total_responses_sent_by_status")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Increment the number of requests received by 1.
		totalRequestsReceived.Add(1)

		// Use httpsnoop to capture metrics while passing along the original response writer.
		metrics := httpsnoop.CaptureMetrics(next, w, r)

		// Increment the total responses sent.
		totalResponsesSent.Add(1)
		// Increment the processing time.
		totalProcessingTimeMicroseconds.Add(metrics.Duration.Microseconds())
		// Increment the count for the response status code.
		totalResponsesSentByStatus.Add(strconv.Itoa(metrics.Code), 1)
	})
}
