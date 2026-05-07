package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

// newLLMTestApp builds a minimal *application wired with an in-memory zap
// observer plus the three LLM streaming knobs the test wants to exercise.
// All other application fields are intentionally zero — LLMStream only
// reads cfg.llm and app.logger, so anything else would just be noise.
func newLLMTestApp(t *testing.T, totalBudget, idleTimeout time.Duration, retries int) (*application, *observer.ObservedLogs) {
	t.Helper()
	core, logs := observer.New(zap.InfoLevel)
	app := &application{logger: zap.New(core)}
	app.config.llm.totalBudget = totalBudget
	app.config.llm.idleTimeout = idleTimeout
	app.config.llm.maxRetriesBeforeFirstByte = retries
	return app, logs
}

// writeSSEChunk emits one OpenAI/SambaNova-style SSE line and flushes the
// response writer so the body chunk is on the wire before the next call.
// Test handlers use this to drip-feed chunks instead of buffering the
// whole response, which is what makes the streaming assertions meaningful.
func writeSSEChunk(t *testing.T, w http.ResponseWriter, payload string) {
	t.Helper()
	fl, ok := w.(http.Flusher)
	if !ok {
		t.Fatal("http.ResponseWriter does not implement http.Flusher; cannot stream")
	}
	if _, err := fmt.Fprintf(w, "data: %s\n\n", payload); err != nil {
		// Mid-stream client disconnects produce write errors here, which
		// some tests deliberately trigger; ignore so the handler can
		// finish and let the client-side assertion be the source of truth.
		return
	}
	fl.Flush()
}

// TestLLMStream_HappyPath_StreamsAndAccumulates is the headline case: the
// stream must (a) accumulate every content delta into LLMStreamResult.Text,
// (b) fire onChunk synchronously per delta in arrival order, (c) capture
// the final-chunk Usage block, and (d) record the terminal FinishReason.
// Any of those failing would invalidate the future SSE handler's contract.
func TestLLMStream_HappyPath_StreamsAndAccumulates(t *testing.T) {
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		writeSSEChunk(t, w, `{"choices":[{"index":0,"delta":{"content":"Hello, "},"finish_reason":null}]}`)
		writeSSEChunk(t, w, `{"choices":[{"index":0,"delta":{"content":"world"},"finish_reason":null}]}`)
		writeSSEChunk(t, w, `{"choices":[{"index":0,"delta":{"content":"!"},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":3,"total_tokens":13}}`)
	}))
	defer srv.Close()

	app, _ := newLLMTestApp(t, 5*time.Second, 1*time.Second, 0)
	var deltas []string
	res, err := app.LLMStream(context.Background(), srv.URL, nil, `{}`,
		func(d string) error {
			deltas = append(deltas, d)
			return nil
		})
	if err != nil {
		t.Fatalf("LLMStream returned error: %v", err)
	}
	if got, want := res.Text, "Hello, world!"; got != want {
		t.Errorf("Text = %q, want %q", got, want)
	}
	if !reflect.DeepEqual(deltas, []string{"Hello, ", "world", "!"}) {
		t.Errorf("onChunk deltas = %v, want [\"Hello, \", \"world\", \"!\"]", deltas)
	}
	if res.FinishReason != "stop" {
		t.Errorf("FinishReason = %q, want %q", res.FinishReason, "stop")
	}
	if res.Usage == nil {
		t.Fatal("Usage = nil; want token usage from final chunk")
	}
	if res.Usage.TotalTokens != 13 {
		t.Errorf("Usage.TotalTokens = %d, want 13", res.Usage.TotalTokens)
	}
	if got := attempts.Load(); got != 1 {
		t.Errorf("server saw %d attempts, want exactly 1", got)
	}
}

// TestLLMStream_StampsRequestIDFromContext is the correlation guard: a
// request originating from an inbound HTTP request must forward its
// X-Request-ID so a Loki query can stitch the inbound and LLM-call lines
// by the same key. Same contract as TestGETRequest_StampsRequestIDFromContext.
func TestLLMStream_StampsRequestIDFromContext(t *testing.T) {
	var captured atomic.Pointer[http.Request]
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured.Store(r.Clone(r.Context()))
		writeSSEChunk(t, w, `{"choices":[{"index":0,"delta":{"content":"x"},"finish_reason":"stop"}]}`)
	}))
	defer srv.Close()

	app, _ := newLLMTestApp(t, 5*time.Second, 1*time.Second, 0)
	ctx := ctxWithRequestID("llm-trace-1")
	if _, err := app.LLMStream(ctx, srv.URL, nil, `{}`, nil); err != nil {
		t.Fatalf("LLMStream: %v", err)
	}
	gotReq := captured.Load()
	if gotReq == nil {
		t.Fatal("server never saw a request")
	}
	if got := gotReq.Header.Get("X-Request-ID"); got != "llm-trace-1" {
		t.Errorf("X-Request-ID forwarded = %q, want %q", got, "llm-trace-1")
	}
}

// TestLLMStream_HandlesDoneSentinel asserts that "data: [DONE]" terminates
// the loop. To make the assertion strong, the handler writes a phantom
// chunk *after* [DONE] which must NOT appear in Text. If the sentinel is
// ignored the test catches it; if the sentinel works, Text == "hi".
func TestLLMStream_HandlesDoneSentinel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeSSEChunk(t, w, `{"choices":[{"index":0,"delta":{"content":"hi"},"finish_reason":null}]}`)
		writeSSEChunk(t, w, `[DONE]`)
		writeSSEChunk(t, w, `{"choices":[{"index":0,"delta":{"content":"PHANTOM_SHOULD_NOT_APPEAR"},"finish_reason":null}]}`)
	}))
	defer srv.Close()

	app, _ := newLLMTestApp(t, 1*time.Second, 500*time.Millisecond, 0)
	res, err := app.LLMStream(context.Background(), srv.URL, nil, `{}`, nil)
	if err != nil {
		t.Fatalf("LLMStream: %v", err)
	}
	if res.Text != "hi" {
		t.Errorf("Text = %q, want %q (phantom chunk after [DONE] leaked)", res.Text, "hi")
	}
}

// TestLLMStream_IgnoresMalformedLines proves that a non-JSON keepalive or
// transient garbage line in the middle of a stream does NOT fail the
// whole call. Real upstream providers periodically send blank pings or
// recover from bad bytes; the consumer has to be tolerant.
func TestLLMStream_IgnoresMalformedLines(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeSSEChunk(t, w, `{"choices":[{"index":0,"delta":{"content":"a"},"finish_reason":null}]}`)
		writeSSEChunk(t, w, `not-json-at-all`)
		writeSSEChunk(t, w, `{"choices":[{"index":0,"delta":{"content":"b"},"finish_reason":"stop"}]}`)
	}))
	defer srv.Close()

	app, _ := newLLMTestApp(t, 1*time.Second, 500*time.Millisecond, 0)
	res, err := app.LLMStream(context.Background(), srv.URL, nil, `{}`, nil)
	if err != nil {
		t.Fatalf("LLMStream: %v", err)
	}
	if res.Text != "ab" {
		t.Errorf("Text = %q, want %q", res.Text, "ab")
	}
}

// TestLLMStream_IdleTimeoutFires is the operational protection guarantee:
// when the upstream sends headers and one chunk, then goes silent, the
// call must abort within idle+epsilon (NOT the full wallclock budget).
// We assert both the sentinel error and the elapsed wall time so a
// regression that swaps the idle timer for a budget-only path fails loudly.
func TestLLMStream_IdleTimeoutFires(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeSSEChunk(t, w, `{"choices":[{"index":0,"delta":{"content":"first"},"finish_reason":null}]}`)
		// Stall longer than the test's idle timeout but well within the
		// wallclock budget. The handler exits when the client cancels.
		select {
		case <-time.After(2 * time.Second):
		case <-r.Context().Done():
		}
	}))
	defer srv.Close()

	const idle = 200 * time.Millisecond
	app, _ := newLLMTestApp(t, 5*time.Second, idle, 0)

	start := time.Now()
	res, err := app.LLMStream(context.Background(), srv.URL, nil, `{}`, nil)
	elapsed := time.Since(start)

	if !errors.Is(err, errLLMIdleTimeout) {
		t.Fatalf("err = %v, want errLLMIdleTimeout", err)
	}
	if res.Text != "first" {
		t.Errorf("Text = %q, want partial %q (bytes that did stream must be preserved)", res.Text, "first")
	}
	// Generous bound: idle timer + scheduling + network noise + ctx
	// propagation should resolve well inside 1s for a 200ms idle window.
	if elapsed > 1*time.Second {
		t.Errorf("idle timeout took %v; want < 1s for idle=%v", elapsed, idle)
	}
}

// TestLLMStream_NoRetryAfterFirstByte is the retry-budget guarantee: once
// any chunk has streamed, a mid-stream connection drop must NOT re-send
// the prompt. Replaying a 30s prompt evaluation for a transient TCP blip
// would double user-visible latency and double our token bill. The retry
// budget is intentionally generous (3) to prove the rule, not the count.
func TestLLMStream_NoRetryAfterFirstByte(t *testing.T) {
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.Header().Set("Content-Type", "text/event-stream")
		writeSSEChunk(t, w, `{"choices":[{"index":0,"delta":{"content":"partial"},"finish_reason":null}]}`)
		// Hijack and close to simulate a mid-stream TCP drop. We can't use
		// a normal handler return here because that would emit a clean EOF
		// which the client's scanner treats as "stream finished cleanly".
		hj, ok := w.(http.Hijacker)
		if !ok {
			t.Errorf("server does not support hijack; cannot simulate mid-stream drop")
			return
		}
		conn, _, err := hj.Hijack()
		if err != nil {
			return
		}
		_ = conn.Close()
	}))
	defer srv.Close()

	app, _ := newLLMTestApp(t, 5*time.Second, 1*time.Second, 3 /*retries; intentionally generous to prove they don't fire*/)
	res, err := app.LLMStream(context.Background(), srv.URL, nil, `{}`, nil)
	if err == nil {
		t.Fatal("want error from mid-stream drop, got nil")
	}
	if res.Text != "partial" {
		t.Errorf("Text = %q, want %q (bytes that did stream must be preserved)", res.Text, "partial")
	}
	if got := attempts.Load(); got != 1 {
		t.Errorf("attempts = %d, want exactly 1 (no retry permitted after first byte)", got)
	}
}

// TestLLMStream_BoundedPreFirstByteRetries is the symmetric guarantee:
// pre-first-byte errors (5xx, dial fail) DO get retried, but bounded
// strictly to cfg.llm.maxRetriesBeforeFirstByte. The handler returns 503
// every time so retryablehttp will exhaust its budget; we assert exactly
// retries+1 attempts.
func TestLLMStream_BoundedPreFirstByteRetries(t *testing.T) {
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	const wantRetries = 2
	app, _ := newLLMTestApp(t, 30*time.Second, 5*time.Second, wantRetries)
	if _, err := app.LLMStream(context.Background(), srv.URL, nil, `{}`, nil); err == nil {
		t.Fatal("want error from exhausted retries, got nil")
	}
	if got := int(attempts.Load()); got != wantRetries+1 {
		t.Errorf("attempts = %d, want %d (retries=%d + 1 initial)", got, wantRetries+1, wantRetries)
	}
}

// TestLLMStream_WallclockBudgetCap proves the budget context actually caps
// the call. The handler holds the response open well beyond the budget;
// the call must return within the budget plus a small scheduling margin,
// not eight seconds later.
func TestLLMStream_WallclockBudgetCap(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(8 * time.Second):
			w.WriteHeader(http.StatusOK)
		case <-r.Context().Done():
		}
	}))
	defer srv.Close()

	const budget = 250 * time.Millisecond
	app, _ := newLLMTestApp(t, budget, 5*time.Second, 0)

	start := time.Now()
	_, err := app.LLMStream(context.Background(), srv.URL, nil, `{}`, nil)
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("want error from budget exhaustion, got nil")
	}
	if elapsed > 1*time.Second {
		t.Errorf("budget enforcement took %v; want < 1s for budget=%v", elapsed, budget)
	}
}

// TestLLMStream_OnChunkErrorAborts asserts that a non-nil return from the
// onChunk callback short-circuits the stream and surfaces that error to
// the caller, while preserving the partial Text. This is the contract the
// future SSE handler relies on: a browser that disconnects mid-stream
// should let the handler return cleanly without forcing the LLM call to
// run to completion.
func TestLLMStream_OnChunkErrorAborts(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeSSEChunk(t, w, `{"choices":[{"index":0,"delta":{"content":"a"},"finish_reason":null}]}`)
		writeSSEChunk(t, w, `{"choices":[{"index":0,"delta":{"content":"b"},"finish_reason":null}]}`)
		writeSSEChunk(t, w, `{"choices":[{"index":0,"delta":{"content":"c"},"finish_reason":"stop"}]}`)
	}))
	defer srv.Close()

	app, _ := newLLMTestApp(t, 5*time.Second, 1*time.Second, 0)
	sentinel := errors.New("client lost")
	var deltas []string
	res, err := app.LLMStream(context.Background(), srv.URL, nil, `{}`,
		func(d string) error {
			deltas = append(deltas, d)
			if d == "b" {
				return sentinel
			}
			return nil
		})
	if !errors.Is(err, sentinel) {
		t.Fatalf("err = %v, want %v", err, sentinel)
	}
	if res.Text != "ab" {
		t.Errorf("Text = %q, want %q (stream stopped after delta 'b')", res.Text, "ab")
	}
	if len(deltas) != 2 {
		t.Errorf("onChunk fired %d times, want exactly 2 (chunk after abort must not fire)", len(deltas))
	}
}

// TestLLMStream_5xxLogsAtErrorLevel: a non-2xx response from the upstream
// is operator-actionable, so the outbound log line must come out at Error
// level. retryablehttp will retry internally, but LLMStream emits exactly
// one log line per call (the final outcome), so this assertion holds
// regardless of internal retries.
func TestLLMStream_5xxLogsAtErrorLevel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	app, logs := newLLMTestApp(t, 30*time.Second, 5*time.Second, 0)
	if _, err := app.LLMStream(context.Background(), srv.URL, nil, `{}`, nil); err == nil {
		t.Fatal("want error from 500 response, got nil")
	}
	errEntries := logs.FilterLevelExact(zap.ErrorLevel).FilterMessage("http outbound").All()
	if len(errEntries) != 1 {
		t.Errorf("want 1 Error-level outbound log entry, got %d", len(errEntries))
	}
}

// TestLLMStream_ContextCancellationLogsAtInfoLevel: when the inbound
// caller's ctx is canceled mid-stream (browser tab closed), the outbound
// log line must NOT be Error. Otherwise every drive-by user disconnect
// generates spurious alert noise. Same policy as the GETRequest sibling test.
func TestLLMStream_ContextCancellationLogsAtInfoLevel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Drip-feed chunks slowly so the client has time to cancel.
		for i := 0; i < 50; i++ {
			select {
			case <-r.Context().Done():
				return
			default:
			}
			writeSSEChunk(t, w, `{"choices":[{"index":0,"delta":{"content":"x"},"finish_reason":null}]}`)
			time.Sleep(50 * time.Millisecond)
		}
	}))
	defer srv.Close()

	app, logs := newLLMTestApp(t, 30*time.Second, 5*time.Second, 0)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(150 * time.Millisecond)
		cancel()
	}()
	if _, err := app.LLMStream(ctx, srv.URL, nil, `{}`, nil); err == nil {
		t.Fatal("want error from canceled ctx, got nil")
	}

	if got := len(logs.FilterLevelExact(zap.ErrorLevel).FilterMessage("http outbound").All()); got != 0 {
		t.Errorf("ctx cancellation logged at Error level (would create alert noise); want 0, got %d", got)
	}
	if got := len(logs.FilterLevelExact(zap.InfoLevel).FilterMessage("http outbound").All()); got != 1 {
		t.Errorf("want exactly 1 Info-level outbound log line on ctx cancel, got %d", got)
	}
}

// TestLLMRequest_BackCompatWrapper is the smoke check that the legacy
// signature still works. Existing buildLLMRequestHelper callers in
// ai_operations.go go through this wrapper, so a regression here would
// silently break the portfolio analysis path.
func TestLLMRequest_BackCompatWrapper(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeSSEChunk(t, w, `{"choices":[{"index":0,"delta":{"content":"hello"},"finish_reason":"stop"}]}`)
	}))
	defer srv.Close()

	app, _ := newLLMTestApp(t, 5*time.Second, 1*time.Second, 0)
	got, err := app.LLMRequest(context.Background(), srv.URL, nil, `{}`)
	if err != nil {
		t.Fatalf("LLMRequest: %v", err)
	}
	if got != "hello" {
		t.Errorf("LLMRequest result = %q, want %q", got, "hello")
	}
}
