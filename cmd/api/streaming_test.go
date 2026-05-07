package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// fakeFlusher counts Flush invocations so tests can assert that the SSE
// writer flushes exactly once per event. Standalone struct rather than
// piggybacking on http.Flusher's interface so it works against a plain
// io.Writer in the same test, not just an http.ResponseWriter.
type fakeFlusher struct{ calls atomic.Int32 }

func (f *fakeFlusher) Flush() { f.calls.Add(1) }

// errWriter always returns errBoom; used to verify writeSSEEvent
// surfaces the underlying transport error rather than swallowing it.
type errWriter struct{ err error }

func (e *errWriter) Write(_ []byte) (int, error) { return 0, e.err }

// TestSetSSEHeaders pins the four header values the SSE protocol +
// nginx ingress depend on. Adding/removing a header here is a wire
// contract change.
func TestSetSSEHeaders(t *testing.T) {
	rec := httptest.NewRecorder()
	setSSEHeaders(rec.Header())

	cases := map[string]string{
		"Content-Type":      "text/event-stream",
		"Cache-Control":     "no-cache",
		"Connection":        "keep-alive",
		"X-Accel-Buffering": "no",
	}
	for name, want := range cases {
		if got := rec.Header().Get(name); got != want {
			t.Errorf("header %q = %q, want %q", name, got, want)
		}
	}
}

// TestWriteSSEEvent_DefaultEvent: the default ("message") event must
// omit the `event:` line so EventSource's onmessage handler picks it
// up. Any leading `event: ` would route it to addEventListener("",..)
// instead and the client would silently drop it.
func TestWriteSSEEvent_DefaultEvent(t *testing.T) {
	var out strings.Builder
	fl := &fakeFlusher{}

	if err := writeSSEEvent(&out, fl, "", "hello"); err != nil {
		t.Fatalf("writeSSEEvent: %v", err)
	}
	if got, want := out.String(), "data: hello\n\n"; got != want {
		t.Errorf("frame = %q, want %q", got, want)
	}
	if fl.calls.Load() != 1 {
		t.Errorf("flush called %d times, want 1", fl.calls.Load())
	}
}

// TestWriteSSEEvent_NamedEvent: named events must prepend the
// `event: <name>\n` line. The client's addEventListener("done",..) is
// the only thing that fires on the structured terminal payload, so a
// missing event name would silently drop the analyzed-portfolio block.
func TestWriteSSEEvent_NamedEvent(t *testing.T) {
	var out strings.Builder
	fl := &fakeFlusher{}

	if err := writeSSEEvent(&out, fl, "done", "{}"); err != nil {
		t.Fatalf("writeSSEEvent: %v", err)
	}
	if got, want := out.String(), "event: done\ndata: {}\n\n"; got != want {
		t.Errorf("frame = %q, want %q", got, want)
	}
}

// TestWriteSSEEvent_MultilineData: the SSE spec says multi-line data
// must be split into one `data:` line per source line. If we instead
// write a literal newline inside the value, the client treats the
// blank line that follows as the event terminator and the second half
// arrives as part of the *next* event — so this is a correctness
// guarantee, not a style preference.
func TestWriteSSEEvent_MultilineData(t *testing.T) {
	var out strings.Builder
	fl := &fakeFlusher{}

	if err := writeSSEEvent(&out, fl, "", "line one\nline two"); err != nil {
		t.Fatalf("writeSSEEvent: %v", err)
	}
	if got, want := out.String(), "data: line one\ndata: line two\n\n"; got != want {
		t.Errorf("frame = %q, want %q", got, want)
	}
}

// TestWriteSSEEvent_PropagatesWriteError: a client disconnect surfaces
// as a write error here, and the per-delta callback uses that as its
// "stop streaming" signal. If we swallowed it, LLMStream would keep
// pulling chunks from SambaNova long after the client was gone.
func TestWriteSSEEvent_PropagatesWriteError(t *testing.T) {
	want := errors.New("client gone")
	if err := writeSSEEvent(&errWriter{err: want}, &fakeFlusher{}, "", "x"); !errors.Is(err, want) {
		t.Errorf("err = %v, want %v", err, want)
	}
}

// TestWriteSSEEventJSON_MarshalsPayload: the structured `done` and
// `error` events go through this path. Asserts both the framing (event
// line, JSON body, blank line) and that the JSON itself round-trips so
// future field additions stay parseable.
func TestWriteSSEEventJSON_MarshalsPayload(t *testing.T) {
	var out strings.Builder
	fl := &fakeFlusher{}
	payload := map[string]any{"finish_reason": "stop", "usage": map[string]int{"total_tokens": 42}}

	if err := writeSSEEventJSON(&out, fl, "done", payload); err != nil {
		t.Fatalf("writeSSEEventJSON: %v", err)
	}
	frame := out.String()
	if !strings.HasPrefix(frame, "event: done\ndata: ") {
		t.Errorf("frame missing event line, got %q", frame)
	}
	if !strings.HasSuffix(frame, "\n\n") {
		t.Errorf("frame missing terminal blank line, got %q", frame)
	}
	body := strings.TrimSuffix(strings.TrimPrefix(frame, "event: done\ndata: "), "\n\n")
	var got map[string]any
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatalf("payload not JSON: %v\n%s", err, body)
	}
	if got["finish_reason"] != "stop" {
		t.Errorf("finish_reason = %v, want stop", got["finish_reason"])
	}
}

// TestClassifyLLMStreamError table: the wire-level error message is
// part of the public contract — clients display it. Any change to the
// strings is a UX-visible change. The defaults branch must NEVER leak
// raw transport errors (they can carry hostnames, retry counts, etc.)
// so the unknown-error case has a fixed phrase.
func TestClassifyLLMStreamError(t *testing.T) {
	cases := []struct {
		name string
		in   error
		want string
	}{
		{"idle timeout", errLLMIdleTimeout, "upstream stalled"},
		{"deadline exceeded", context.DeadlineExceeded, "upstream timed out"},
		{"canceled", context.Canceled, "request canceled"},
		{"non-2xx (llm: prefix)", errors.New("llm: non-2xx response: 503"), "llm: non-2xx response: 503"},
		{"random transport err", errors.New("dial tcp 10.0.0.5:443: connection refused"), "internal stream error"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := classifyLLMStreamError(tc.in); got != tc.want {
				t.Errorf("classifyLLMStreamError(%v) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestStreamLLMToSSE_HappyPath is the end-to-end glue test for the
// LLM→SSE bridge. It wires a fake SambaNova upstream that emits three
// content deltas and verifies the JSON-wrapped wire format the
// browser EventSource is going to consume:
//
//	data: {"delta":"Hello, "}
//	data: {"delta":"world"}
//	data: {"delta":"!"}
//
// (each followed by the terminal blank line). It also asserts the
// returned LLMStreamResult carries the joined text and the final-chunk
// usage block, since the handler downstream needs both for the `done`
// event payload.
func TestStreamLLMToSSE_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		writeSSEChunk(t, w, `{"choices":[{"index":0,"delta":{"content":"Hello, "},"finish_reason":null}]}`)
		writeSSEChunk(t, w, `{"choices":[{"index":0,"delta":{"content":"world"},"finish_reason":null}]}`)
		writeSSEChunk(t, w, `{"choices":[{"index":0,"delta":{"content":"!"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":3,"total_tokens":4}}`)
	}))
	defer srv.Close()

	app, _ := newLLMTestApp(t, 5*time.Second, 1*time.Second, 0)
	rec := httptest.NewRecorder()
	fl, ok := http.ResponseWriter(rec).(http.Flusher)
	if !ok {
		t.Fatal("httptest.ResponseRecorder does not implement http.Flusher")
	}

	res, err := app.streamLLMToSSE(context.Background(), rec, fl, srv.URL, nil, `{}`)
	if err != nil {
		t.Fatalf("streamLLMToSSE: %v", err)
	}

	if got, want := res.Text, "Hello, world!"; got != want {
		t.Errorf("res.Text = %q, want %q", got, want)
	}
	if res.Usage == nil || res.Usage.TotalTokens != 4 {
		t.Errorf("res.Usage = %+v, want TotalTokens=4", res.Usage)
	}

	// Decode each SSE event into its delta and assert order.
	deltas := readSSEDeltas(t, rec.Body.String())
	want := []string{"Hello, ", "world", "!"}
	if len(deltas) != len(want) {
		t.Fatalf("emitted %d deltas, want %d. body=%q", len(deltas), len(want), rec.Body.String())
	}
	for i, d := range deltas {
		if d != want[i] {
			t.Errorf("delta[%d] = %q, want %q", i, d, want[i])
		}
	}
}

// TestStreamLLMToSSE_UpstreamError exercises the failure mode where
// SambaNova returns 503 with retries=0 (so no replay). streamLLMToSSE
// must surface the error to its caller (the handler will then frame
// it as an SSE error event); it must NOT itself emit a partial event,
// because at the moment of failure no delta has been written yet and
// inserting a synthetic "" delta would corrupt the client's view.
func TestStreamLLMToSSE_UpstreamError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	app, _ := newLLMTestApp(t, 2*time.Second, 1*time.Second, 0)
	rec := httptest.NewRecorder()
	fl, _ := http.ResponseWriter(rec).(http.Flusher)

	_, err := app.streamLLMToSSE(context.Background(), rec, fl, srv.URL, nil, `{}`)
	if err == nil {
		t.Fatal("expected error from streamLLMToSSE on 503 upstream, got nil")
	}
	if rec.Body.Len() != 0 {
		t.Errorf("expected no SSE body on upstream error, got %q", rec.Body.String())
	}
}

// readSSEDeltas extracts the {"delta": "..."} payloads from an SSE
// response body in arrival order. Tolerates the named-event lines and
// blank separators per the SSE spec; a parse failure on any data line
// fails the test loudly because that means the wire format drifted.
func readSSEDeltas(t *testing.T, body string) []string {
	t.Helper()
	var deltas []string
	sc := bufio.NewScanner(strings.NewReader(body))
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		raw := strings.TrimPrefix(line, "data: ")
		var parsed struct {
			Delta string `json:"delta"`
		}
		if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
			// Non-delta JSON (the named events) — skip rather than
			// fail. The test only asserts on delta payloads.
			continue
		}
		deltas = append(deltas, parsed.Delta)
	}
	if err := sc.Err(); err != nil && !errors.Is(err, io.EOF) {
		t.Fatalf("scanner: %v", err)
	}
	return deltas
}
