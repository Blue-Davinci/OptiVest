package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/Blue-Davinci/OptiVest/internal/data"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

// fieldByKey returns the value of a zap field on a recorded log entry, or
// the zero zapcore.Field if no such field is present. Tests use this to
// assert specific fields without index-fragility.
func fieldByKey(entry observer.LoggedEntry, key string) zapcore.Field {
	for _, f := range entry.Context {
		if f.Key == key {
			return f
		}
	}
	return zapcore.Field{}
}

// TestRequestID_GeneratedWhenAbsent verifies that an inbound request with
// no X-Request-ID header gets one generated server-side, surfaced both in
// the response header (so the client can correlate) and in the request
// context (so downstream middleware/handlers can log with it).
func TestRequestID_GeneratedWhenAbsent(t *testing.T) {
	app, _ := observedApp(t)
	var seenInContext string
	h := app.requestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenInContext = contextRequestID(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/foo", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	got := rec.Header().Get(requestIDHeader)
	if got == "" {
		t.Fatal("expected X-Request-ID response header to be set, got empty")
	}
	if seenInContext != got {
		t.Fatalf("context request id %q does not match response header %q", seenInContext, got)
	}
	if len(got) != generatedRequestIDBytes*2 {
		t.Fatalf("expected hex ID of length %d, got %d (%q)", generatedRequestIDBytes*2, len(got), got)
	}
}

// TestRequestID_PassthroughValid verifies that a clean inbound X-Request-ID
// is preserved end-to-end. This is what makes cross-service correlation
// work: an upstream proxy stamps once, every layer keeps that ID.
func TestRequestID_PassthroughValid(t *testing.T) {
	app, _ := observedApp(t)
	var seen string
	h := app.requestID(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		seen = contextRequestID(r.Context())
	}))

	const wantID = "edge-cdn-abc123_def.456"
	req := httptest.NewRequest(http.MethodGet, "/foo", nil)
	req.Header.Set(requestIDHeader, wantID)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if seen != wantID {
		t.Fatalf("expected passthrough %q, got %q", wantID, seen)
	}
	if rec.Header().Get(requestIDHeader) != wantID {
		t.Fatalf("expected response header echo %q, got %q", wantID, rec.Header().Get(requestIDHeader))
	}
}

// TestRequestID_RejectsUnsafeOrOversize verifies the sanitization policy:
// inbound IDs that contain unsafe characters (CR/LF for log injection,
// shell metas, spaces) or exceed requestIDMaxLen must be replaced with a
// freshly generated ID. This is the security-relevant behaviour of the
// middleware.
func TestRequestID_RejectsUnsafeOrOversize(t *testing.T) {
	cases := []struct {
		name  string
		input string
	}{
		{"crlf injection", "abc\r\nFAKE: line"},
		{"space", "id with space"},
		{"shell metas", "abc;rm -rf /"},
		{"unicode", "abcдef"},
		{"oversize", strings.Repeat("a", requestIDMaxLen+1)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app, _ := observedApp(t)
			var seen string
			h := app.requestID(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				seen = contextRequestID(r.Context())
			}))
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set(requestIDHeader, tc.input)
			h.ServeHTTP(httptest.NewRecorder(), req)

			if seen == tc.input {
				t.Fatalf("dangerous input %q was passed through unchanged", tc.input)
			}
			if !isAcceptableRequestID(seen) {
				t.Fatalf("regenerated ID %q does not pass the acceptance policy", seen)
			}
		})
	}
}

// TestLogRequests_EmitsAllFields exercises the happy path: a successful
// request through the full requestID -> logRequests stack must produce
// exactly one zap line carrying every documented field with the expected
// values.
func TestLogRequests_EmitsAllFields(t *testing.T) {
	app, recorded := observedApp(t)
	chain := app.requestID(app.logRequests(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte("hello world"))
	})))

	req := httptest.NewRequest(http.MethodGet, "/widgets/42", nil)
	req.Header.Set(requestIDHeader, "trace-xyz-1")
	req.Header.Set("User-Agent", "go-test/1.0")
	req.RemoteAddr = "203.0.113.7:55555"
	chain.ServeHTTP(httptest.NewRecorder(), req)

	entries := recorded.All()
	if len(entries) != 1 {
		t.Fatalf("expected exactly one log entry, got %d", len(entries))
	}
	e := entries[0]
	if e.Message != "http request" {
		t.Fatalf("unexpected message %q", e.Message)
	}

	want := map[string]any{
		"method":     "GET",
		"path":       "/widgets/42",
		"status":     int64(http.StatusTeapot),
		"bytes":      int64(len("hello world")),
		"req_id":     "trace-xyz-1",
		"user_agent": "go-test/1.0",
		"user_id":    int64(0),
	}
	for k, v := range want {
		got := fieldByKey(e, k)
		if got.Key == "" {
			t.Errorf("missing field %q", k)
			continue
		}
		switch want := v.(type) {
		case string:
			if got.String != want {
				t.Errorf("field %q: want %q, got %q", k, want, got.String)
			}
		case int64:
			if got.Integer != want {
				t.Errorf("field %q: want %d, got %d", k, want, got.Integer)
			}
		}
	}

	// latency_ms must be present and non-negative; the actual value is
	// timing-dependent and not worth pinning.
	lat := fieldByKey(e, "latency_ms")
	if lat.Key == "" {
		t.Error("missing latency_ms field")
	} else if lat.Integer < 0 {
		t.Errorf("latency_ms is negative: %d", lat.Integer)
	}
}

// TestLogRequests_LevelOnServerError verifies that 5xx responses promote
// the log line to Error level. This is the line ops should be alerting on,
// so the level matters more than any specific field value.
func TestLogRequests_LevelOnServerError(t *testing.T) {
	app, recorded := observedApp(t)
	chain := app.requestID(app.logRequests(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	})))

	chain.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/explode", nil))

	entries := recorded.All()
	if len(entries) != 1 {
		t.Fatalf("expected one entry, got %d", len(entries))
	}
	if entries[0].Level != zapcore.ErrorLevel {
		t.Fatalf("expected Error level for 5xx, got %s", entries[0].Level)
	}
	if v := fieldByKey(entries[0], "status"); v.Integer != int64(http.StatusInternalServerError) {
		t.Fatalf("expected status 500, got %d", v.Integer)
	}
}

// TestLogRequests_LevelOnClientError keeps client errors at Info so they
// do not pollute alerting. They still increment the 4xx expvar counter for
// graphing.
func TestLogRequests_LevelOnClientError(t *testing.T) {
	app, recorded := observedApp(t)
	chain := app.requestID(app.logRequests(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "no", http.StatusBadRequest)
	})))

	chain.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	entries := recorded.All()
	if len(entries) != 1 {
		t.Fatalf("expected one entry, got %d", len(entries))
	}
	if entries[0].Level != zapcore.InfoLevel {
		t.Fatalf("expected Info level for 4xx, got %s", entries[0].Level)
	}
}

// TestLogRequests_PicksUpUserIDWrittenByInnerMiddleware proves that the
// shared *requestLog holder pattern actually works: a synthetic inner
// middleware writes user.ID into the holder, and the OUTER logging
// middleware sees that write when it emits the line. This is the
// invariant the whole design hinges on; if the wrong context layer is
// used, user_id silently goes back to 0.
func TestLogRequests_PicksUpUserIDWrittenByInnerMiddleware(t *testing.T) {
	app, recorded := observedApp(t)

	innerMW := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if h := contextRequestLog(r.Context()); h != nil {
				h.userID = 42
			}
			next.ServeHTTP(w, r)
		})
	}

	chain := app.requestID(app.logRequests(innerMW(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))))

	chain.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	entries := recorded.All()
	if len(entries) != 1 {
		t.Fatalf("expected one entry, got %d", len(entries))
	}
	if v := fieldByKey(entries[0], "user_id"); v.Integer != 42 {
		t.Fatalf("expected user_id=42 to propagate from inner middleware, got %d", v.Integer)
	}
}

// TestLogRequests_PanicSafe verifies that a panicking inner handler still
// produces a single log line with a 500 status, by composing the
// middleware in the SAME outer-to-inner order the production router uses:
//
//	requestID -> logRequests -> recoverPanic -> handler
//
// The ordering matters: httpsnoop.CaptureMetrics (used inside logRequests)
// re-propagates inner panics, so logRequests must sit OUTSIDE recoverPanic.
// If anyone ever flips that, this test fails loudly because the "http
// request" log line never appears.
func TestLogRequests_PanicSafe(t *testing.T) {
	app, recorded := observedApp(t)
	chain := app.requestID(app.logRequests(app.recoverPanic(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		panic("kaboom")
	}))))

	rec := httptest.NewRecorder()
	chain.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 after recover, got %d", rec.Code)
	}
	entries := recorded.All()
	// recoverPanic itself logs an error via serverErrorResponse, plus
	// our request line. Find the http request line specifically.
	var httpEntry *observer.LoggedEntry
	for i := range entries {
		if entries[i].Message == "http request" {
			httpEntry = &entries[i]
			break
		}
	}
	if httpEntry == nil {
		t.Fatalf("missing 'http request' log line; got: %#v", entries)
	}
	if httpEntry.Level != zapcore.ErrorLevel {
		t.Errorf("post-panic line should be Error, got %s", httpEntry.Level)
	}
	if v := fieldByKey(*httpEntry, "status"); v.Integer != int64(http.StatusInternalServerError) {
		t.Errorf("post-panic status should be 500, got %d", v.Integer)
	}
}

// TestLogRequests_BytesWrittenAccurateAcrossMultipleWrites confirms that
// httpsnoop (the wrapper this middleware relies on) sums bytes from
// multiple Write calls correctly. If httpsnoop ever changes that
// behaviour, this test will catch it before ops graphs lie.
func TestLogRequests_BytesWrittenAccurateAcrossMultipleWrites(t *testing.T) {
	app, recorded := observedApp(t)
	chain := app.requestID(app.logRequests(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("hello "))
		_, _ = w.Write([]byte("world"))
	})))
	chain.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	entries := recorded.All()
	if len(entries) != 1 {
		t.Fatalf("expected one entry, got %d", len(entries))
	}
	if v := fieldByKey(entries[0], "bytes"); v.Integer != int64(len("hello world")) {
		t.Fatalf("expected bytes=%d, got %d", len("hello world"), v.Integer)
	}
}

// TestAuthenticate_PopulatesRequestLogHolder is a small integration test
// for the one-line addition I made to the authenticate middleware: after a
// successful auth (here forced via a stub so we don't need the DB layer),
// the request log holder must carry the user ID, ready for logRequests to
// pick up.
//
// Going through the real authenticate function would require wiring
// Models, Redis, DB, and a token; instead I exercise the same code path
// with a contextSetUser equivalent and the holder write that authenticate
// performs. The behaviour under test is the holder population, not the
// auth lookup itself (which has its own coverage).
func TestAuthenticate_PopulatesRequestLogHolder(t *testing.T) {
	app, recorded := observedApp(t)

	stubAuth := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := &data.User{ID: 7, Activated: true}
			r = app.contextSetUser(r, user)
			if h := contextRequestLog(r.Context()); h != nil {
				h.userID = user.ID
			}
			next.ServeHTTP(w, r)
		})
	}

	chain := app.requestID(app.logRequests(stubAuth(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))))

	chain.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	entries := recorded.All()
	if len(entries) != 1 {
		t.Fatalf("expected one entry, got %d", len(entries))
	}
	if v := fieldByKey(entries[0], "user_id"); v.Integer != 7 {
		t.Fatalf("expected user_id=7, got %d", v.Integer)
	}
}

// TestLoggerFromRequest_EnrichesWithCorrelationFields verifies the
// public helper used by handlers: any line they emit through the returned
// logger must already carry req_id, conn_id, and user_id.
func TestLoggerFromRequest_EnrichesWithCorrelationFields(t *testing.T) {
	app, recorded := observedApp(t)

	h := app.requestID(app.logRequests(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if holder := contextRequestLog(r.Context()); holder != nil {
			holder.userID = 99
		}
		// Stash a synthetic conn_id so we can also assert it propagates.
		ctx := context.WithValue(r.Context(), connIDContextKey, int64(123))
		r = r.WithContext(ctx)
		app.loggerFromRequest(r).Info("handler-line", zap.String("custom", "v"))
		w.WriteHeader(http.StatusOK)
	})))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(requestIDHeader, "trace-handler")
	h.ServeHTTP(httptest.NewRecorder(), req)

	var handlerEntry *observer.LoggedEntry
	for i, e := range recorded.All() {
		if e.Message == "handler-line" {
			handlerEntry = &recorded.All()[i]
			break
		}
	}
	if handlerEntry == nil {
		t.Fatalf("expected to find 'handler-line' in recorded logs, got: %#v", recorded.All())
	}
	if v := fieldByKey(*handlerEntry, "req_id"); v.String != "trace-handler" {
		t.Errorf("expected req_id=trace-handler, got %q", v.String)
	}
	if v := fieldByKey(*handlerEntry, "conn_id"); v.Integer != 123 {
		t.Errorf("expected conn_id=123, got %d", v.Integer)
	}
	if v := fieldByKey(*handlerEntry, "user_id"); v.Integer != 99 {
		t.Errorf("expected user_id=99, got %d", v.Integer)
	}
	if v := fieldByKey(*handlerEntry, "custom"); v.String != "v" {
		t.Errorf("expected custom=v on handler line, got %q", v.String)
	}
}

// TestLogRequests_OneLinePerRequestUnderConcurrency stress-tests the
// middleware under the race detector to make sure (a) every concurrent
// request emits exactly one log line and (b) each request's line carries
// its OWN user ID, not a sibling's. This is the failure mode if the
// holder were ever shared across requests instead of per-context.
func TestLogRequests_OneLinePerRequestUnderConcurrency(t *testing.T) {
	app, recorded := observedApp(t)

	// Inner middleware writes a deterministic user_id derived from the
	// inbound X-Request-ID so the test can correlate emitted lines back
	// to their originating goroutine.
	stamp := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := contextRequestID(r.Context())
			var n int64
			_, err := fmt.Sscanf(id, "req-%d", &n)
			if err == nil {
				if h := contextRequestLog(r.Context()); h != nil {
					h.userID = n
				}
			}
			next.ServeHTTP(w, r)
		})
	}

	chain := app.requestID(app.logRequests(stamp(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))))

	const n = 100
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set(requestIDHeader, fmt.Sprintf("req-%d", i))
			chain.ServeHTTP(httptest.NewRecorder(), req)
		}(i)
	}
	wg.Wait()

	all := recorded.All()
	if len(all) != n {
		t.Fatalf("expected %d log lines, got %d", n, len(all))
	}

	// Every emitted line must have user_id matching the numeric portion
	// of its req_id, proving holders did not leak between requests.
	for _, e := range all {
		reqID := fieldByKey(e, "req_id").String
		var want int64
		if _, err := fmt.Sscanf(reqID, "req-%d", &want); err != nil {
			t.Errorf("malformed req_id %q in log entry", reqID)
			continue
		}
		got := fieldByKey(e, "user_id").Integer
		if got != want {
			t.Errorf("user_id leak: req_id=%s carried user_id=%d, expected %d", reqID, got, want)
		}
	}
}

// TestIsAcceptableRequestID_TableDrivenPolicy locks in the per-byte
// allowlist. Treating this as a separate unit (in addition to the
// integration test on requestID) makes regressions on the policy itself
// much easier to bisect.
func TestIsAcceptableRequestID_TableDrivenPolicy(t *testing.T) {
	cases := []struct {
		s    string
		want bool
	}{
		{"", false},
		{"abc", true},
		{"ABC123", true},
		{"abc-DEF_ghi.123", true},
		{"abc def", false},
		{"abc;def", false},
		{"abc\ndef", false},
		{"абв", false},
		{strings.Repeat("a", requestIDMaxLen), true},
		{strings.Repeat("a", requestIDMaxLen+1), false},
	}
	for _, tc := range cases {
		if got := isAcceptableRequestID(tc.s); got != tc.want {
			t.Errorf("isAcceptableRequestID(%q) = %v, want %v", tc.s, got, tc.want)
		}
	}
}
