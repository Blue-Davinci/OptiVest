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
	"time"

	"github.com/Blue-Davinci/OptiVest/internal/data"
	"github.com/hashicorp/go-retryablehttp"
	"github.com/microcosm-cc/bluemonday"
	"go.uber.org/zap"
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
// caller's RetryMax so the existing reliability behaviour is preserved.
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
// is set on the ctx, and a single structured log line is emitted summarising
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
// JSON-marshalled body or a pre-built multipart bytes.Buffer. Same context
// propagation, header stamping, and structured logging behaviour as
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

// LLMRequest sends a context-bound POST to the SambaNova chat-completions
// endpoint and reads the streaming response, accumulating chunk content into
// a single string. ctx propagates so a client disconnect cancels the
// long-running upstream stream promptly. The same outbound-correlation
// behaviour as GETRequest/POSTRequest applies: X-Request-ID is forwarded
// when present, and one structured log line summarises the call.
//
// The streaming reader uses a 1 MiB scanner buffer because SambaNova chunks
// can exceed bufio.Scanner's default 64 KiB limit on long responses.
func (app *application) LLMRequest(ctx context.Context, requestURL string, headers map[string]string, body string) (string, error) {
	log := app.loggerFromContext(ctx)
	start := time.Now()
	jsonBody := []byte(body)
	clientC := NewClient(60*time.Second, 3, app.logger)

	req, err := retryablehttp.NewRequest(http.MethodPost, requestURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		logOutbound(log, http.MethodPost, requestURL, 0, 0, start, ctx, err)
		return "", err
	}
	req = req.WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	stampOutboundRequestID(req, ctx)

	resp, err := clientC.httpClient.Do(req)
	if err != nil {
		logOutbound(log, http.MethodPost, requestURL, 0, 0, start, ctx, err)
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		err := errors.New("non-2xx response code")
		logOutbound(log, http.MethodPost, requestURL, resp.StatusCode, 0, start, ctx, err)
		return "", err
	}

	var fullResponse strings.Builder
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || line == "data: " {
			continue
		}
		// Defensive prefix strip - older code relied on a fixed length
		// of 6, which would panic on a malformed line shorter than that.
		line = strings.TrimPrefix(line, "data: ")

		var chunk Chunk
		if err := json.Unmarshal([]byte(line), &chunk); err != nil {
			continue
		}
		for _, choice := range chunk.Choices {
			if choice.Delta.Content != "" {
				fullResponse.WriteString(choice.Delta.Content)
			}
		}
		// finishReason on any choice ends the stream early.
		done := false
		for _, choice := range chunk.Choices {
			if choice.FinishReason != nil {
				done = true
				break
			}
		}
		if done {
			break
		}
	}

	if err := scanner.Err(); err != nil {
		logOutbound(log, http.MethodPost, requestURL, resp.StatusCode, int64(fullResponse.Len()), start, ctx, err)
		return "", err
	}

	logOutbound(log, http.MethodPost, requestURL, resp.StatusCode, int64(fullResponse.Len()), start, ctx, nil)
	return fullResponse.String(), nil
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
