package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"expvar"
	"net/http"
	"time"

	"github.com/felixge/httpsnoop"
	"github.com/tomasen/realip"
	"go.uber.org/zap"
)

// requestIDHeader is the canonical HTTP header for cross-service request
// correlation. Inbound values are accepted (so a CDN or upstream proxy can
// stamp an ID once and have it propagate end-to-end), with sanitization to
// prevent log-injection.
const requestIDHeader = "X-Request-ID"

// requestIDMaxLen caps how many bytes I trust from an inbound X-Request-ID
// header. 128 bytes comfortably covers UUIDs, ULIDs, snowflakes, and most
// custom schemes while bounding the cost of an attacker spamming huge IDs
// into our log pipeline.
const requestIDMaxLen = 128

// generatedRequestIDBytes controls the entropy of the server-side fallback
// ID. 8 bytes hex-encoded gives 16 readable chars and 64 bits of entropy,
// which is enough to make collisions essentially impossible at our request
// volume while keeping log lines compact.
const generatedRequestIDBytes = 8

// requestLog metrics. These complement the existing rate_limiter_* and
// portfolio_* counters by exposing per-request observability that operators
// can graph from /debug/vars without touching the log pipeline:
//   - request_log_total              total requests observed by logRequests
//   - request_log_5xx_total          subset that returned a server error
//   - request_log_4xx_total          subset that returned a client error
//   - request_id_generated_total     subset that did not arrive with an ID
//   - request_id_rejected_total      inbound IDs rejected for being too long
//     or containing illegal characters
var (
	requestLogTotal    = expvar.NewInt("request_log_total")
	requestLog5xxTotal = expvar.NewInt("request_log_5xx_total")
	requestLog4xxTotal = expvar.NewInt("request_log_4xx_total")
	requestIDGenerated = expvar.NewInt("request_id_generated_total")
	requestIDRejected  = expvar.NewInt("request_id_rejected_total")
)

// requestID middleware stamps every inbound request with a stable
// correlation ID, available via contextRequestID(r.Context()) for the rest
// of the chain and echoed back to the client in the X-Request-ID response
// header so they can include it in bug reports.
//
// Behavior:
//   - If the request arrives with X-Request-ID, validate and reuse it.
//   - If not, or if the inbound value is empty/oversized/contains an
//     unsafe character, generate a fresh ID via crypto/rand.
//   - The accepted-or-generated ID is set on the response header before
//     the next handler runs, so a panic recovery still surfaces the ID.
//
// Sanitization: I accept ASCII letters, digits, dash, underscore, dot.
// Anything else triggers regeneration. This blocks log-injection
// (CR/LF/quotes), keeps the field grep-friendly, and is permissive enough
// to cover UUID, ULID, snowflake, and most custom IDs without surprises.
func (app *application) requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(requestIDHeader)
		if id == "" {
			id = generateRequestID()
			requestIDGenerated.Add(1)
		} else if !isAcceptableRequestID(id) {
			id = generateRequestID()
			requestIDRejected.Add(1)
		}

		w.Header().Set(requestIDHeader, id)
		ctx := context.WithValue(r.Context(), requestIDContextKey, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// generateRequestID returns a hex-encoded random ID. crypto/rand is
// overkill for collision-avoidance but trivially cheap (single syscall on
// Linux) and removes any "is this random enough?" concerns. If the OS RNG
// fails I fall back to a timestamp-based ID rather than panic; the request
// is still answerable, the ID is just less unique.
func generateRequestID() string {
	buf := make([]byte, generatedRequestIDBytes)
	if _, err := rand.Read(buf); err != nil {
		// Extremely unlikely; degrade gracefully. The nanosecond timestamp
		// is unique enough that two simultaneous failures still produce
		// distinct IDs in practice.
		return "ts-" + time.Now().UTC().Format("20060102T150405.000000000")
	}
	return hex.EncodeToString(buf)
}

// isAcceptableRequestID enforces the sanitization policy described on
// requestID. Length cap first (cheap), then per-byte allowlist.
func isAcceptableRequestID(s string) bool {
	if len(s) == 0 || len(s) > requestIDMaxLen {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z':
		case c >= 'A' && c <= 'Z':
		case c >= '0' && c <= '9':
		case c == '-' || c == '_' || c == '.':
		default:
			return false
		}
	}
	return true
}

// logRequests middleware emits exactly one structured zap line per HTTP
// request, after the request has been fully served. Wraps the response
// writer via httpsnoop so I can capture the status code and bytes written
// without re-implementing the WriteHeader/Write tap dance.
//
// Fields emitted:
//
//	method        e.g. GET, POST
//	path          r.URL.Path (deliberately not RequestURI; query strings
//	              can contain tokens or PII)
//	status        final HTTP status code
//	bytes         response body bytes written
//	latency_ms    integer milliseconds (graphable directly)
//	remote_ip     real client IP via tomasen/realip (same source the
//	              limiter uses, for consistent correlation)
//	req_id        the value set by requestID middleware
//	conn_id       per-connection ID stamped by ConnContext (server.go)
//	user_id       0 if the request never authenticated, else user.ID
//	user_agent    truncated at 256 bytes; helpful triage signal
//
// Log levels:
//
//	2xx, 3xx -> Info
//	4xx      -> Info (still expected; spammy clients show as 4xx volume)
//	5xx      -> Error (this is the line ops should be paging on)
//
// Placement: this middleware sits OUTSIDE rateLimit/authenticate so that
// 401s and 429s still produce a log line. authenticate writes the user ID
// into the requestLog holder (see contextRequestLog) which I read here.
func (app *application) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Stash a mutable holder in the context BEFORE next.ServeHTTP so
		// inner middleware can populate it and have those writes visible
		// when I read the holder back here. WithValue gives downstream
		// middleware/handlers a different request struct, but the pointer
		// stored as the value is shared, which is exactly what I want.
		holder := &requestLog{}
		ctx := context.WithValue(r.Context(), requestLogContextKey, holder)
		r = r.WithContext(ctx)

		metrics := httpsnoop.CaptureMetrics(next, w, r)

		requestLogTotal.Add(1)
		switch {
		case metrics.Code >= 500:
			requestLog5xxTotal.Add(1)
		case metrics.Code >= 400:
			requestLog4xxTotal.Add(1)
		}

		ua := r.UserAgent()
		if len(ua) > 256 {
			ua = ua[:256]
		}

		fields := []zap.Field{
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path),
			zap.Int("status", metrics.Code),
			zap.Int64("bytes", metrics.Written),
			zap.Int64("latency_ms", metrics.Duration.Milliseconds()),
			zap.String("remote_ip", realip.FromRequest(r)),
			zap.String("req_id", contextRequestID(r.Context())),
			zap.Int64("conn_id", contextConnID(r.Context())),
			zap.Int64("user_id", holder.userID),
			zap.String("user_agent", ua),
		}

		if metrics.Code >= 500 {
			app.logger.Error("http request", fields...)
		} else {
			app.logger.Info("http request", fields...)
		}
	})
}

// loggerFromRequest returns a zap logger pre-enriched with this request's
// correlation fields (req_id, conn_id, user_id). Handlers that want their
// per-request logs to participate in the same correlation should prefer
// this over reading app.logger directly.
//
// I intentionally do NOT cache the enriched logger on the application
// struct: handlers may call this after authenticate has populated the
// holder, so the user_id should be read fresh on every call.
func (app *application) loggerFromRequest(r *http.Request) *zap.Logger {
	return app.loggerFromContext(r.Context())
}

// loggerFromContext is the context-only sibling of loggerFromRequest.
// Background helpers (e.g. anything in investment_operations.go that
// receives a ctx but no *http.Request) call this so their log lines still
// carry req_id/conn_id/user_id when the ctx originated from an HTTP
// request. If the ctx never flowed through the request middleware (e.g.
// a cron job or a test using context.Background), the missing fields
// resolve to their zero values and the line is still emitted, just
// without correlation - same behavior as app.logger directly.
func (app *application) loggerFromContext(ctx context.Context) *zap.Logger {
	var userID int64
	if holder := contextRequestLog(ctx); holder != nil {
		userID = holder.userID
	}
	return app.logger.With(
		zap.String("req_id", contextRequestID(ctx)),
		zap.Int64("conn_id", contextConnID(ctx)),
		zap.Int64("user_id", userID),
	)
}
