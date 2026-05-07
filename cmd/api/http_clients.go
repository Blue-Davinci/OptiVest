package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"github.com/hashicorp/go-retryablehttp"
	"github.com/microcosm-cc/bluemonday"
	"go.uber.org/zap"

	"github.com/Blue-Davinci/OptiVest/internal/data"
)

// Optivet_Client is the central HTTP client wrapper used for every outbound
// call. It owns a retryable client plus an optional logger; when the logger
// is set, GETRequest/POSTRequest emit one structured line per call so an
// inbound request can be traced through to its specific upstream hop.
type Optivet_Client struct {
	httpClient *retryablehttp.Client
	logger     *zap.Logger
}

// NewClient builds an Optivet_Client. The logger is optional; pass nil to
// suppress outbound tracing. The retry policy stays linear-jitter with the
// caller's RetryMax so the existing reliability behavior is preserved.
func NewClient(timeout time.Duration, retries int, logger *zap.Logger) *Optivet_Client {
	retryClient := retryablehttp.NewClient()
	retryClient.RetryMax = retries
	retryClient.HTTPClient.Timeout = timeout
	retryClient.Backoff = retryablehttp.LinearJitterBackoff
	retryClient.ErrorHandler = retryablehttp.PassthroughErrorHandler
	retryClient.Logger = nil

	return &Optivet_Client{
		httpClient: retryClient,
		logger:     logger,
	}
}

// outboundRequestIDHeader is the canonical correlation header. The inbound
// requestID middleware accepts the same name; using it on outbound preserves
// the same ID end-to-end if the upstream service forwards it (e.g. internal
// services). External providers ignore it, which is fine.
const outboundRequestIDHeader = "X-Request-ID"

// outboundLoggerFromContext returns a per-call enriched logger derived from
// the client's base logger. Returns nil when the client was constructed
// without a logger so callers can short-circuit cheaply.
func (c *Optivet_Client) outboundLoggerFromContext(ctx context.Context) *zap.Logger {
	if c == nil || c.logger == nil {
		return nil
	}
	var userID int64
	if holder := contextRequestLog(ctx); holder != nil {
		userID = holder.userID
	}
	return c.logger.With(
		zap.String("req_id", contextRequestID(ctx)),
		zap.Int64("conn_id", contextConnID(ctx)),
		zap.Int64("user_id", userID),
	)
}

// stampOutboundRequestID copies the inbound correlation ID onto the outbound
// request. Skips the write when the ctx never flowed through the request
// middleware (background work, tests) or when the caller has already set
// the header explicitly.
func stampOutboundRequestID(req *retryablehttp.Request, ctx context.Context) {
	if req == nil {
		return
	}
	if req.Header.Get(outboundRequestIDHeader) != "" {
		return
	}
	id := contextRequestID(ctx)
	if id == "" {
		return
	}
	req.Header.Set(outboundRequestIDHeader, id)
}

// safeHostPath parses the URL and returns just the host and path. The full
// URL is deliberately NOT logged because several upstream providers carry
// API keys as query-string parameters, and any token we surface in logs is
// effectively a leak. See SECURITY.md.
func safeHostPath(rawURL string) (host, path string) {
	u, err := url.Parse(rawURL)
	if err != nil || u == nil {
		return "", ""
	}
	return u.Host, u.Path
}

// logOutbound emits one structured log line per outbound call. Levels:
//   - status >= 500           Error (operator should page on this)
//   - transport / non-ctx err Error
//   - ctx canceled / deadline Info  (expected; user disconnected or our
//     timeout fired - not an upstream fault)
//   - everything else         Info
//
// The log shape mirrors the inbound logRequests middleware (method, status,
// bytes, latency_ms, req_id, conn_id, user_id) so a single Loki/Grafana
// query can stitch an inbound line to its outbound children.
func logOutbound(log *zap.Logger, method, rawURL string, status int, bytes int64, start time.Time, ctx context.Context, callErr error) {
	if log == nil {
		return
	}
	host, path := safeHostPath(rawURL)
	fields := []zap.Field{
		zap.String("method", method),
		zap.String("host", host),
		zap.String("path", path),
		zap.Int("status", status),
		zap.Int64("bytes", bytes),
		zap.Int64("latency_ms", time.Since(start).Milliseconds()),
	}
	if callErr != nil {
		fields = append(fields, zap.String("err", callErr.Error()))
	}

	switch {
	case errors.Is(ctx.Err(), context.Canceled), errors.Is(ctx.Err(), context.DeadlineExceeded):
		log.Info("http outbound", fields...)
	case status >= 500, callErr != nil:
		log.Error("http outbound", fields...)
	default:
		log.Info("http outbound", fields...)
	}
}

// GETRequest sends a context-bound GET to the specified URL and unmarshals
// the JSON response into T. The caller's ctx is propagated to the underlying
// http.Request, so a client disconnect or upstream timeout cancels the call
// promptly. The inbound X-Request-ID is forwarded on the outbound when one
// is set on the ctx, and a single structured log line is emitted summarizing
// the call (see logOutbound for the schema).
func GETRequest[T any](ctx context.Context, c *Optivet_Client, requestURL string, headers map[string]string) (T, error) {
	var result T
	log := c.outboundLoggerFromContext(ctx)
	start := time.Now()

	req, err := retryablehttp.NewRequest(http.MethodGet, requestURL, nil)
	if err != nil {
		logOutbound(log, http.MethodGet, requestURL, 0, 0, start, ctx, err)
		return result, err
	}
	req = req.WithContext(ctx)
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	stampOutboundRequestID(req, ctx)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		logOutbound(log, http.MethodGet, requestURL, 0, 0, start, ctx, err)
		return result, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		message := fmt.Sprintf("non-2xx response code: %d | url: %s", resp.StatusCode, requestURL)
		err := errors.New(message)
		logOutbound(log, http.MethodGet, requestURL, resp.StatusCode, 0, start, ctx, err)
		return result, err
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logOutbound(log, http.MethodGet, requestURL, resp.StatusCode, 0, start, ctx, err)
		return result, err
	}

	if err := json.Unmarshal(body, &result); err != nil {
		logOutbound(log, http.MethodGet, requestURL, resp.StatusCode, int64(len(body)), start, ctx, err)
		return result, err
	}

	logOutbound(log, http.MethodGet, requestURL, resp.StatusCode, int64(len(body)), start, ctx, nil)
	return result, nil
}

// POSTRequest sends a context-bound POST to the specified URL with either a
// JSON-marshaled body or a pre-built multipart bytes.Buffer. Same context
// propagation, header stamping, and structured logging behavior as
// GETRequest.
func POSTRequest[T any](ctx context.Context, c *Optivet_Client, requestURL string, headers map[string]string, body interface{}, isMultipart bool) (T, error) {
	var result T
	log := c.outboundLoggerFromContext(ctx)
	start := time.Now()

	var reqBody io.Reader
	if isMultipart {
		bufferBody, ok := body.(bytes.Buffer)
		if !ok {
			err := fmt.Errorf("expected body to be bytes.Buffer for multipart request")
			logOutbound(log, http.MethodPost, requestURL, 0, 0, start, ctx, err)
			return result, err
		}
		reqBody = &bufferBody
	} else {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			logOutbound(log, http.MethodPost, requestURL, 0, 0, start, ctx, err)
			return result, err
		}
		reqBody = bytes.NewBuffer(jsonBody)
	}

	req, err := retryablehttp.NewRequest(http.MethodPost, requestURL, reqBody)
	if err != nil {
		logOutbound(log, http.MethodPost, requestURL, 0, 0, start, ctx, err)
		return result, err
	}
	req = req.WithContext(ctx)
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	stampOutboundRequestID(req, ctx)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		logOutbound(log, http.MethodPost, requestURL, 0, 0, start, ctx, err)
		return result, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		err := fmt.Errorf("non-2xx response code: %d", resp.StatusCode)
		logOutbound(log, http.MethodPost, requestURL, resp.StatusCode, 0, start, ctx, err)
		return result, err
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		logOutbound(log, http.MethodPost, requestURL, resp.StatusCode, 0, start, ctx, err)
		return result, err
	}

	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		logOutbound(log, http.MethodPost, requestURL, resp.StatusCode, int64(len(bodyBytes)), start, ctx, err)
		return result, err
	}

	logOutbound(log, http.MethodPost, requestURL, resp.StatusCode, int64(len(bodyBytes)), start, ctx, nil)
	return result, nil
}

// LLMStreamResult is the structured outcome of an LLMStream call.
//
//   - Text holds the concatenation of every content delta the upstream
//     emitted. Even when an error is returned, Text contains whatever did
//     stream successfully — partial-progress callers may want to log it
//     or surface it to the user before falling back.
//   - Usage carries the final-chunk token-usage block when the upstream
//     emitted one (stream_options.include_usage = true on the request).
//     Nil otherwise.
//   - FinishReason holds the terminal finish_reason from the last choice
//     ("stop", "length", "content_filter", ...). Empty string when the
//     stream ended without one (EOF, idle timeout, ctx cancel).
type LLMStreamResult struct {
	Text         string
	Usage        *LLMUsage
	FinishReason string
}

// errLLMIdleTimeout is returned by LLMStream when no SSE chunk arrived
// within the configured idle window. The underlying transport error is
// always context.Canceled (we cancel the stream ctx from the idle timer);
// translating it to this sentinel lets operators distinguish "upstream
// stalled" from "user disconnected" without having to inspect log fields.
var errLLMIdleTimeout = errors.New("llm: idle timeout exceeded waiting for next chunk")

// idleTimeoutReader wraps an io.Reader with a per-Read inactivity deadline.
// When no Read returns bytes within timeout, onIdle is invoked exactly once
// (typically a context cancel func that unblocks the consumer). The reader
// is single-goroutine; SSE consumers always read sequentially, which is
// the only call site here.
type idleTimeoutReader struct {
	r       io.Reader
	timer   *time.Timer
	timeout time.Duration
	fired   atomic.Bool
}

func newIdleTimeoutReader(r io.Reader, timeout time.Duration, onIdle func()) *idleTimeoutReader {
	itr := &idleTimeoutReader{r: r, timeout: timeout}
	if timeout <= 0 {
		return itr
	}
	itr.timer = time.AfterFunc(timeout, func() {
		itr.fired.Store(true)
		if onIdle != nil {
			onIdle()
		}
	})
	return itr
}

func (i *idleTimeoutReader) Read(p []byte) (int, error) {
	n, err := i.r.Read(p)
	// Re-arm only when bytes arrived and the timer has not already fired.
	// Once fired, the cancel signal is in flight and we must not "rescue"
	// the request by extending the deadline — even if more bytes raced
	// in before the cancel propagated.
	if i.timer != nil && n > 0 && !i.fired.Load() {
		i.timer.Reset(i.timeout)
	}
	return n, err
}

func (i *idleTimeoutReader) Stop() {
	if i.timer != nil {
		i.timer.Stop()
	}
}

func (i *idleTimeoutReader) FiredIdle() bool { return i.fired.Load() }

// llmStreamingDefaults returns the per-call streaming knobs, falling back
// to safe values when cfg.llm is unset (e.g. zero-valued application{} in
// tests). Production wiring is in main.go via the -llm-* flags.
func (app *application) llmStreamingDefaults() (totalBudget, idleTimeout time.Duration, retries int) {
	totalBudget = app.config.llm.totalBudget
	if totalBudget <= 0 {
		totalBudget = 90 * time.Second
	}
	idleTimeout = app.config.llm.idleTimeout
	if idleTimeout <= 0 {
		idleTimeout = 15 * time.Second
	}
	retries = app.config.llm.maxRetriesBeforeFirstByte
	if retries < 0 {
		retries = 2
	}
	return
}

// LLMStream sends a context-bound POST to the SambaNova chat-completions
// endpoint and consumes the SSE response chunk-by-chunk. Three behavioral
// guarantees on top of the basic POST path:
//
//  1. A wallclock budget context (cfg.llm.totalBudget, default 90s) caps
//     total time across pre-first-byte retries plus the streaming read.
//     The retry layer's backoff sleeps inherit this deadline, so a tight
//     budget naturally compresses the retry window.
//  2. An idle deadline (cfg.llm.idleTimeout, default 15s) aborts the call
//     when no chunk has arrived within the window. Protects connection
//     slots from a stalled upstream that has sent headers but no body,
//     and caps the impact of a slowloris-style upstream.
//  3. retryablehttp.RetryMax is bounded to cfg.llm.maxRetriesBeforeFirstByte
//     (default 2). Retries fire only on dial / TLS / non-2xx errors that
//     happen *before* the response body is streamed; mid-stream errors
//     are surfaced verbatim because replaying a 30s prompt for a transient
//     blip costs more (latency + tokens) than failing fast.
//
// onChunk, when non-nil, is invoked synchronously for every non-empty
// content delta the upstream emits — wire it through to a Server-Sent
// Events handler to stream tokens to the browser, or pass nil to
// collect-only and read the joined Text from the returned LLMStreamResult.
// Returning a non-nil error from onChunk aborts the stream with that error
// and preserves the partial Text accumulated so far.
//
// X-Request-ID forwarding and the structured outbound log line behave
// identically to GETRequest / POSTRequest.
func (app *application) LLMStream(
	ctx context.Context,
	requestURL string,
	headers map[string]string,
	body string,
	onChunk func(delta string) error,
) (LLMStreamResult, error) {
	log := app.loggerFromContext(ctx)
	start := time.Now()
	var result LLMStreamResult

	totalBudget, idleTimeout, retries := app.llmStreamingDefaults()

	streamCtx, cancel := context.WithTimeout(ctx, totalBudget)
	defer cancel()

	// A streaming-tuned client. No per-request HTTPClient.Timeout — the
	// budget ctx is the cap, and a stdlib timeout would also kill healthy
	// long reads. Constructed fresh per call so a future per-tenant
	// CheckRetry / rate-limiter hook can be stamped without affecting the
	// shared Optivet_Client used by every other outbound path.
	retryClient := retryablehttp.NewClient()
	retryClient.RetryMax = retries
	retryClient.HTTPClient.Timeout = 0
	retryClient.Backoff = retryablehttp.LinearJitterBackoff
	retryClient.ErrorHandler = retryablehttp.PassthroughErrorHandler
	retryClient.Logger = nil
	// Tighten the retry-wait bounds vs. retryablehttp's library defaults
	// (1s / 30s). The LLM call already runs under a wallclock budget, so
	// long backoffs only burn that budget; with RetryMax=2 the hard cap
	// here is 100ms + 200ms = 300ms of cumulative sleep which is plenty
	// to recover from a flapping upstream while staying invisible to the
	// p95 user-visible latency.
	retryClient.RetryWaitMin = 100 * time.Millisecond
	retryClient.RetryWaitMax = 2 * time.Second

	req, err := retryablehttp.NewRequest(http.MethodPost, requestURL, bytes.NewBufferString(body))
	if err != nil {
		logOutbound(log, http.MethodPost, requestURL, 0, 0, start, ctx, err)
		return result, err
	}
	req = req.WithContext(streamCtx)
	req.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	stampOutboundRequestID(req, ctx)

	resp, err := retryClient.Do(req)
	if err != nil {
		// Pass the parent ctx (not streamCtx) so logOutbound classifies
		// wallclock-budget exhaustion as Error rather than Info: a budget
		// blow-out is operator-visible, only a true parent-ctx cancel
		// (user disconnect) should land at Info.
		logOutbound(log, http.MethodPost, requestURL, 0, 0, start, ctx, err)
		return result, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		err := fmt.Errorf("llm: non-2xx response: %d", resp.StatusCode)
		logOutbound(log, http.MethodPost, requestURL, resp.StatusCode, 0, start, ctx, err)
		return result, err
	}

	// Idle timer wraps the body. The cancel func belongs to streamCtx, so
	// firing it propagates through the http transport and unblocks the
	// next scanner.Scan with context.Canceled — which we then translate
	// to errLLMIdleTimeout for clarity.
	idleR := newIdleTimeoutReader(resp.Body, idleTimeout, cancel)
	defer idleR.Stop()

	var fullResponse strings.Builder
	scanner := bufio.NewScanner(idleR)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || line == "data: " {
			continue
		}
		line = strings.TrimPrefix(line, "data: ")
		if line == "[DONE]" {
			break
		}

		var chunk Chunk
		if err := json.Unmarshal([]byte(line), &chunk); err != nil {
			// Non-JSON keepalive or transient malformed line; ignore
			// rather than failing the whole stream over one byte.
			continue
		}
		if chunk.Usage != nil {
			result.Usage = chunk.Usage
		}

		for _, choice := range chunk.Choices {
			if choice.Delta.Content != "" {
				fullResponse.WriteString(choice.Delta.Content)
				if onChunk != nil {
					if cbErr := onChunk(choice.Delta.Content); cbErr != nil {
						result.Text = fullResponse.String()
						logOutbound(log, http.MethodPost, requestURL, resp.StatusCode, int64(fullResponse.Len()), start, ctx, cbErr)
						return result, cbErr
					}
				}
			}
			if choice.FinishReason != nil {
				result.FinishReason = *choice.FinishReason
			}
		}
		if result.FinishReason != "" {
			break
		}
	}

	if scanErr := scanner.Err(); scanErr != nil {
		result.Text = fullResponse.String()
		out := scanErr
		if idleR.FiredIdle() {
			out = errLLMIdleTimeout
		}
		logOutbound(log, http.MethodPost, requestURL, resp.StatusCode, int64(fullResponse.Len()), start, ctx, out)
		return result, out
	}

	result.Text = fullResponse.String()
	logOutbound(log, http.MethodPost, requestURL, resp.StatusCode, int64(fullResponse.Len()), start, ctx, nil)
	return result, nil
}

// LLMRequest is preserved as a thin back-compat wrapper around LLMStream.
// Existing callers (buildLLMRequestHelper) only need the joined string;
// future SSE handlers should call LLMStream directly with a non-nil
// onChunk to forward partial deltas.
func (app *application) LLMRequest(ctx context.Context, requestURL string, headers map[string]string, body string) (string, error) {
	res, err := app.LLMStream(ctx, requestURL, headers, body, nil)
	return res.Text, err
}

// scraperGetRSSFeeds fetches a single feed URL and decodes the (RSS or Atom)
// XML body. ctx flows from the cron-driven caller so a process shutdown
// promptly aborts in-flight scrapes. The structured outbound log line lives
// here directly because the body is XML, not JSON, so this path cannot reuse
// GETRequest[T any].
func (app *application) scraperGetRSSFeeds(ctx context.Context, retryMax, clientTimeout int, requestURL string, sanitizer *bluemonday.Policy) (*data.RSSFeed, error) {
	log := app.loggerFromContext(ctx)
	start := time.Now()
	retryClient := NewClient(
		time.Duration(clientTimeout)*time.Second,
		retryMax,
		app.logger,
	)
	const responseContextTimeout = 30 * time.Second

	req, err := retryablehttp.NewRequest(http.MethodGet, requestURL, nil)
	if err != nil {
		logOutbound(log, http.MethodGet, requestURL, 0, 0, start, ctx, err)
		return nil, err
	}
	scraperCtx, cancel := context.WithTimeout(ctx, responseContextTimeout)
	defer cancel()
	req = req.WithContext(scraperCtx)
	stampOutboundRequestID(req, ctx)

	resp, err := retryClient.httpClient.Do(req)
	if err != nil {
		logOutbound(log, http.MethodGet, requestURL, 0, 0, start, scraperCtx, err)
		switch {
		case strings.Contains(err.Error(), "context deadline exceeded"):
			return nil, data.ErrContextDeadline
		default:
			return nil, err
		}
	}
	defer resp.Body.Close()

	rssFeed := &data.RSSFeed{}
	err = app.RssFeedDecoderDecider(requestURL, rssFeed, sanitizer, resp)
	if err != nil {
		logOutbound(log, http.MethodGet, requestURL, resp.StatusCode, 0, start, scraperCtx, err)
		switch {
		case strings.Contains(err.Error(), "context deadline exceeded"):
			return nil, data.ErrContextDeadline
		case strings.Contains(err.Error(), "feed type"):
			return &data.RSSFeed{RetryMax: int32(retryMax), StatusCode: int32(resp.StatusCode)}, data.ErrUnableToDetectFeedType
		default:
			return nil, err
		}
	}

	logOutbound(log, http.MethodGet, requestURL, resp.StatusCode, 0, start, scraperCtx, nil)
	return rssFeed, nil
}
