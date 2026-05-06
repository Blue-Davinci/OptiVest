package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

// observedClient builds an Optivet_Client wired to an in-memory zap observer
// so tests can assert on the structured outbound log lines the production
// code emits. Returns the client and the live log capture handle.
func observedClient(t *testing.T) (*Optivet_Client, *observer.ObservedLogs) {
	t.Helper()
	core, logs := observer.New(zap.InfoLevel)
	return NewClient(5*time.Second, 0, zap.New(core)), logs
}

// ctxWithRequestID builds a context populated the same way the requestID
// middleware populates inbound requests. Used by tests to simulate an
// outbound call originating from a real HTTP request.
func ctxWithRequestID(id string) context.Context {
	return context.WithValue(context.Background(), requestIDContextKey, id)
}

// echoServer returns an httptest server that captures the inbound request
// (so tests can assert what the outbound client sent) and replies with a
// trivial JSON object. The captured request pointer is shared, so tests
// must not call ServeHTTP concurrently against the same server.
type echoServer struct {
	srv      *httptest.Server
	captured atomic.Pointer[http.Request]
	status   int
}

func newEchoServer(status int) *echoServer {
	es := &echoServer{status: status}
	es.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Clone so the request stays valid after the handler returns; the
		// test reads from it on a different goroutine when assertions run
		// inline.
		clone := r.Clone(r.Context())
		es.captured.Store(clone)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(es.status)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	return es
}

func (es *echoServer) Close() { es.srv.Close() }

// TestGETRequest_StampsRequestIDFromContext is the headline assertion: when
// the inbound request carries a correlation ID, the outbound call must
// forward it as X-Request-ID so an upstream service (or a log scraper) can
// stitch the two sides of the call together by the same key.
func TestGETRequest_StampsRequestIDFromContext(t *testing.T) {
	es := newEchoServer(http.StatusOK)
	defer es.Close()

	c, _ := observedClient(t)
	ctx := ctxWithRequestID("abc123def456")

	type body struct {
		Ok bool `json:"ok"`
	}
	got, err := GETRequest[body](ctx, c, es.srv.URL+"/v1/foo", nil)
	if err != nil {
		t.Fatalf("GETRequest returned error: %v", err)
	}
	if !got.Ok {
		t.Fatalf("decode failed: got %+v", got)
	}
	captured := es.captured.Load()
	if captured == nil {
		t.Fatal("server never saw a request")
	}
	if got := captured.Header.Get("X-Request-ID"); got != "abc123def456" {
		t.Errorf("X-Request-ID forwarded = %q, want %q", got, "abc123def456")
	}
}

// TestGETRequest_DoesNotStampWhenContextHasNoID is the symmetric guard:
// background work (cron, tests, anything not flowing through the request
// middleware) must NOT inject an empty X-Request-ID header. An empty header
// is still a header, and downstream services treating it as "this caller is
// trying to spoof correlation" would be entirely justified.
func TestGETRequest_DoesNotStampWhenContextHasNoID(t *testing.T) {
	es := newEchoServer(http.StatusOK)
	defer es.Close()

	c, _ := observedClient(t)

	type body struct {
		Ok bool `json:"ok"`
	}
	if _, err := GETRequest[body](context.Background(), c, es.srv.URL+"/health", nil); err != nil {
		t.Fatalf("GETRequest returned error: %v", err)
	}
	captured := es.captured.Load()
	if captured == nil {
		t.Fatal("server never saw a request")
	}
	if vals := captured.Header.Values("X-Request-ID"); len(vals) != 0 {
		t.Errorf("X-Request-ID was set despite empty ctx: %v", vals)
	}
}

// TestGETRequest_DoesNotClobberCallerProvidedHeader covers the case where a
// caller has already chosen to set X-Request-ID explicitly (e.g. forwarding
// an externally-supplied trace ID). The helper must respect that choice.
func TestGETRequest_DoesNotClobberCallerProvidedHeader(t *testing.T) {
	es := newEchoServer(http.StatusOK)
	defer es.Close()

	c, _ := observedClient(t)
	ctx := ctxWithRequestID("middleware-id")

	type body struct {
		Ok bool `json:"ok"`
	}
	if _, err := GETRequest[body](ctx, c, es.srv.URL+"/x",
		map[string]string{"X-Request-ID": "caller-supplied-id"}); err != nil {
		t.Fatalf("GETRequest returned error: %v", err)
	}
	captured := es.captured.Load()
	if got := captured.Header.Get("X-Request-ID"); got != "caller-supplied-id" {
		t.Errorf("caller-supplied X-Request-ID was overwritten: got %q", got)
	}
}

// TestGETRequest_EmitsStructuredLogLine asserts the outbound log shape
// matches the inbound logRequests middleware (method, host, path, status,
// bytes, latency_ms, req_id, conn_id, user_id) so a single Loki/Grafana
// query can stitch inbound and outbound lines by req_id.
//
// It also asserts that the log line does NOT contain a "url" field. Several
// upstream providers carry their API key in the query string (Alpha Vantage,
// FRED, FMP), and surfacing the full URL would be a credential leak.
func TestGETRequest_EmitsStructuredLogLine(t *testing.T) {
	es := newEchoServer(http.StatusOK)
	defer es.Close()

	c, logs := observedClient(t)
	ctx := ctxWithRequestID("trace-1")

	type body struct {
		Ok bool `json:"ok"`
	}
	if _, err := GETRequest[body](ctx, c, es.srv.URL+"/v1/timeseries", nil); err != nil {
		t.Fatalf("GETRequest returned error: %v", err)
	}

	entries := logs.FilterMessage("http outbound").All()
	if len(entries) != 1 {
		t.Fatalf("want exactly 1 'http outbound' log entry, got %d", len(entries))
	}
	e := entries[0]
	got := e.ContextMap()

	for _, key := range []string{"method", "host", "path", "status", "bytes", "latency_ms", "req_id", "conn_id", "user_id"} {
		if _, ok := got[key]; !ok {
			t.Errorf("log entry missing field %q", key)
		}
	}
	if got["method"] != "GET" {
		t.Errorf("method = %v, want GET", got["method"])
	}
	if got["path"] != "/v1/timeseries" {
		t.Errorf("path = %v, want /v1/timeseries", got["path"])
	}
	if got["req_id"] != "trace-1" {
		t.Errorf("req_id = %v, want trace-1", got["req_id"])
	}
	if got["status"] != int64(200) {
		t.Errorf("status = %v, want 200", got["status"])
	}
	if _, leaked := got["url"]; leaked {
		t.Error("log entry leaked full url field; query strings can carry API keys (see SECURITY.md)")
	}
}

// TestGETRequest_5xxLogsAtErrorLevel: 5xx responses have to be loud enough
// for an operator to see them. The outbound log line on a 500 must come out
// at Error level so it shows up in the alerting query, mirroring the
// inbound logger's policy.
func TestGETRequest_5xxLogsAtErrorLevel(t *testing.T) {
	es := newEchoServer(http.StatusInternalServerError)
	defer es.Close()

	c, logs := observedClient(t)
	ctx := ctxWithRequestID("trace-err")

	type body struct {
		Ok bool `json:"ok"`
	}
	_, err := GETRequest[body](ctx, c, es.srv.URL+"/v1/sad", nil)
	if err == nil {
		t.Fatal("want error from non-2xx response, got nil")
	}

	errEntries := logs.FilterLevelExact(zap.ErrorLevel).FilterMessage("http outbound").All()
	if len(errEntries) != 1 {
		t.Fatalf("want exactly 1 Error-level outbound log line, got %d (all levels: %d)",
			len(errEntries), logs.Len())
	}
}

// TestGETRequest_ContextCancellation_LogsAtInfoLevel: when the caller's ctx
// is canceled mid-flight (deliberate disconnect), the outbound log line
// must NOT show up as Error. Otherwise every browser-tab-close that lands
// during a slow upstream call generates spurious alert noise. We expect
// the line at Info level instead.
func TestGETRequest_ContextCancellation_LogsAtInfoLevel(t *testing.T) {
	// Slow server: holds the response for longer than the test will wait
	// for it. The handler observes ctx cancellation through r.Context()
	// and returns promptly so the test does not leak the goroutine.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(2 * time.Second):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ok":true}`))
		case <-r.Context().Done():
			return
		}
	}))
	defer server.Close()

	c, logs := observedClient(t)
	parent := ctxWithRequestID("trace-cancel")
	ctx, cancel := context.WithCancel(parent)
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	type body struct {
		Ok bool `json:"ok"`
	}
	if _, err := GETRequest[body](ctx, c, server.URL+"/slow", nil); err == nil {
		t.Fatal("want error from canceled context, got nil")
	}

	errEntries := logs.FilterLevelExact(zap.ErrorLevel).FilterMessage("http outbound").All()
	if len(errEntries) != 0 {
		t.Errorf("ctx cancellation logged at Error level (would create alert noise); want Info")
	}
	infoEntries := logs.FilterLevelExact(zap.InfoLevel).FilterMessage("http outbound").All()
	if len(infoEntries) != 1 {
		t.Errorf("want exactly 1 Info-level outbound log line on ctx cancel, got %d", len(infoEntries))
	}
}

// TestPOSTRequest_StampsRequestIDFromContext mirrors the GET assertion: the
// POST helper must also forward X-Request-ID so the OCR.Space and predictor
// micro-service calls participate in the same correlation pipeline.
func TestPOSTRequest_StampsRequestIDFromContext(t *testing.T) {
	es := newEchoServer(http.StatusOK)
	defer es.Close()

	c, _ := observedClient(t)
	ctx := ctxWithRequestID("post-trace")

	type body struct {
		Ok bool `json:"ok"`
	}
	if _, err := POSTRequest[body](ctx, c, es.srv.URL+"/predict",
		map[string]string{"Content-Type": "application/json"},
		map[string]string{"hello": "world"}, false); err != nil {
		t.Fatalf("POSTRequest returned error: %v", err)
	}
	captured := es.captured.Load()
	if got := captured.Header.Get("X-Request-ID"); got != "post-trace" {
		t.Errorf("X-Request-ID forwarded = %q, want %q", got, "post-trace")
	}
}

// TestSafeHostPath_StripsQueryAndFragment is a direct unit test on the URL
// sanitiser the outbound logger uses. Anything that survives this function
// ends up in production logs, so the contract has to be tight.
func TestSafeHostPath_StripsQueryAndFragment(t *testing.T) {
	cases := []struct {
		raw      string
		wantHost string
		wantPath string
	}{
		{
			raw:      "https://www.alphavantage.co/query?function=TIME_SERIES_DAILY&symbol=IBM&apikey=DO_NOT_LOG_THIS",
			wantHost: "www.alphavantage.co",
			wantPath: "/query",
		},
		{
			raw:      "https://api.fiscaldata.treasury.gov/services/api/v1/series?api_key=secret#frag",
			wantHost: "api.fiscaldata.treasury.gov",
			wantPath: "/services/api/v1/series",
		},
		{
			raw:      "not a url at all",
			wantHost: "",
			wantPath: "not a url at all",
		},
	}
	for _, tc := range cases {
		gotHost, gotPath := safeHostPath(tc.raw)
		if gotHost != tc.wantHost || gotPath != tc.wantPath {
			t.Errorf("safeHostPath(%q) = (%q, %q), want (%q, %q)",
				tc.raw, gotHost, gotPath, tc.wantHost, tc.wantPath)
		}
		if strings.Contains(gotHost+gotPath, "apikey") || strings.Contains(gotHost+gotPath, "api_key") {
			t.Errorf("safeHostPath(%q) leaked api key fragment", tc.raw)
		}
	}
}

// TestNewClient_NilLoggerIsHonoured makes sure constructing a client without
// a logger remains a supported, no-allocation path. Callers that opt out of
// outbound tracing (background scripts, throwaway tests) should not pay for
// the structured logger they did not ask for.
func TestNewClient_NilLoggerIsHonoured(t *testing.T) {
	es := newEchoServer(http.StatusOK)
	defer es.Close()

	c := NewClient(5*time.Second, 0, nil)
	type body struct {
		Ok bool `json:"ok"`
	}
	if _, err := GETRequest[body](context.Background(), c, es.srv.URL+"/", nil); err != nil {
		t.Fatalf("GETRequest with nil logger returned error: %v", err)
	}
}
