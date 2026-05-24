package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"go.uber.org/zap"

	"github.com/Blue-Davinci/OptiVest/internal/data"
)

// SSE protocol notes:
//
// We use Server-Sent Events for the streaming portfolio-analysis endpoint
// because the browser's EventSource API speaks it natively (no extra
// client library, automatic reconnect, automatic UTF-8 framing). The
// endpoint sits inside the regular /v1 router (not the dedicated long-
// lived sseRoutes server) because each call is bounded by the LLM
// streaming budget — typically 10-30s, capped at -llm-total-budget=90s
// — so it behaves much more like a slow HTTP request than a persistent
// push channel. Going through the normal middleware chain means it
// inherits per-IP rate limiting (you do not want a user spamming
// expensive LLM calls), authentication, and the one-line-per-request
// log entry, all of which are correct here.
//
// All events are JSON-wrapped, even per-token deltas, so the wire shape
// stays parseable from a single `JSON.parse(event.data)` on the client
// regardless of whether new fields are added later. Three event names
// are produced:
//
//   default ("message")  - {"delta": "<text>"}             one per LLM chunk
//   "done"               - {"finish_reason", "usage", "analyzed"}
//   "error"              - {"error": "<reason>"}            terminal failure
//
// The default event is anonymous (no `event:` line) so EventSource's
// `onmessage` handler picks it up by default; named events route to
// `addEventListener("done", ...)` / `addEventListener("error", ...)`
// on the client side.

// setSSEHeaders writes the canonical Server-Sent Events response headers.
// Must be called before the first body byte. X-Accel-Buffering disables
// nginx response buffering when this server is behind an nginx ingress —
// without it nginx waits to fill its 4 KiB buffer before flushing, which
// defeats the whole point of streaming.
func setSSEHeaders(h http.Header) {
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("Connection", "keep-alive")
	h.Set("X-Accel-Buffering", "no")
}

// writeSSEEvent emits one SSE event. event="" omits the `event:` line so
// the message arrives on EventSource's default `onmessage` handler. data
// is split on '\n' and each line is prefixed with `data: ` per the SSE
// spec, which keeps multi-line payloads (e.g. JSON-encoded errors with
// newlines from the upstream) parseable without hand-escaping.
//
// The flush is unconditional so the client sees the event as soon as
// it arrives, not whenever the response writer's buffer happens to fill.
// A write error short-circuits and is returned: callers (the handler
// loop, the per-delta callback) use that as the disconnect signal and
// abort the upstream stream cleanly.
func writeSSEEvent(w io.Writer, fl http.Flusher, event, data string) error {
	var buf strings.Builder
	if event != "" {
		buf.WriteString("event: ")
		buf.WriteString(event)
		buf.WriteByte('\n')
	}
	for _, line := range strings.Split(data, "\n") {
		buf.WriteString("data: ")
		buf.WriteString(line)
		buf.WriteByte('\n')
	}
	buf.WriteByte('\n')
	if _, err := io.WriteString(w, buf.String()); err != nil {
		return err
	}
	fl.Flush()
	return nil
}

// writeSSEEventJSON marshals payload to JSON and writes it as a single
// SSE event with the given name. Convenience wrapper for the structured
// `done` and `error` events. Returns the write error verbatim so the
// caller can short-circuit on client disconnect.
func writeSSEEventJSON(w io.Writer, fl http.Flusher, event string, payload any) error {
	blob, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return writeSSEEvent(w, fl, event, string(blob))
}

// classifyLLMStreamError translates an internal LLMStream error into a
// short, user-safe phrase fit for the SSE `error` event. The full
// underlying error stays in the structured server logs (one line per
// outbound call from logOutbound, plus a handler-side log on the error
// path); the wire-level message is deliberately coarse so we never leak
// transport details, hostnames, or anything an attacker could turn into
// a fingerprinting signal.
func classifyLLMStreamError(err error) string {
	switch {
	case errors.Is(err, errLLMIdleTimeout):
		return "upstream stalled"
	case errors.Is(err, context.DeadlineExceeded):
		return "upstream timed out"
	case errors.Is(err, context.Canceled):
		return "request canceled"
	default:
		// Surface only the prefix-stripped sentinel-style messages we
		// emit ourselves (e.g. "llm: non-2xx response: 503"). Anything
		// else collapses to a generic phrase.
		msg := err.Error()
		if strings.HasPrefix(msg, "llm: ") {
			return msg
		}
		return "internal stream error"
	}
}

// streamLLMToSSE drives one chat-completions streaming call against the
// configured upstream (Groq by default) and forwards each SSE chunk's
// content delta to the client as a JSON-wrapped event. It
// is intentionally a single-purpose helper rather than inlined into the
// portfolio handler so the wire-format glue (LLMStream → JSON event →
// flush) is unit-testable against a fake LLM upstream without dragging
// the full DB / auth surface into the test.
//
// Return semantics mirror LLMStream itself: the result is returned in
// all paths (zero-value on early failure), and any non-nil error came
// from either the upstream LLM or the per-delta SSE write. On the
// error path streamLLMToSSE does NOT emit an SSE error event itself —
// the caller has more context (which logical operation was streaming,
// which user, etc.) and is better placed to write the final framed
// error and any persistence-bypass logging.
func (app *application) streamLLMToSSE(
	ctx context.Context,
	w io.Writer,
	fl http.Flusher,
	url string,
	headers map[string]string,
	body string,
) (LLMStreamResult, error) {
	return app.LLMStream(ctx, url, headers, body, func(delta string) error {
		return writeSSEEventJSON(w, fl, "", map[string]string{"delta": delta})
	})
}

// streamInvestmentPortfolioAnalysisHandler serves the streaming variant
// of the existing /investments/analysis endpoint. It runs the same DB
// pre-checks as the non-streaming handler (goals, investment analysis,
// non-empty portfolio), and only switches into SSE mode once those
// 4xx-eligible checks have passed — that way validation failures still
// produce the standard JSON error envelopes clients are used to, and we
// only commit to text/event-stream framing when there is real work to
// stream.
//
// Once streaming starts, content deltas are forwarded to the client as
// fast as the upstream produces them (one anonymous event per delta with
// `{"delta": "..."}`), the joined text is then parsed via the same
// parseLLMResponse used by the synchronous path, the analyzed portfolio
// is persisted via CreateLLMAnalysisResponse (best-effort: a persist
// failure logs but does not abort the user's stream), and a final
// `done` event carries the structured result alongside the LLM's
// finish_reason and token usage.
//
// Errors after stream start surface as `event: error` rather than a
// 4xx/5xx status, because by that point the response headers are
// already on the wire — the client is now reading SSE and would not
// see a status-code change.
func (app *application) streamInvestmentPortfolioAnalysisHandler(w http.ResponseWriter, r *http.Request) {
	user := app.contextGetUser(r)
	log := app.loggerFromRequest(r)

	fl, ok := w.(http.Flusher)
	if !ok {
		app.serverErrorResponse(w, r, errors.New("streaming response writer does not implement http.Flusher"))
		return
	}

	goals, err := app.models.FinancialManager.GetGoalsForUserInvestmentHelper(r.Context(), user.ID)
	if err != nil && !errors.Is(err, data.ErrGeneralRecordNotFound) {
		app.serverErrorResponse(w, r, err)
		return
	}
	if goals == nil || len(goals.Goals) == 0 {
		app.failedValidationResponse(w, r, map[string]string{"goals": "no goals set for user"})
		return
	}

	investmentAnalysis, err := app.models.InvestmentPortfolioManager.GetAllInvestmentsByUserID(r.Context(), user.ID)
	if err != nil && !errors.Is(err, data.ErrGeneralRecordNotFound) {
		app.serverErrorResponse(w, r, err)
		return
	}
	if err := app.performInvestmentPortfolioAnalysis(r.Context(), investmentAnalysis, user); err != nil &&
		!errors.Is(err, data.ErrFailedToGetBondData) {
		app.serverErrorResponse(w, r, err)
		return
	}
	if investmentAnalysis == nil ||
		(len(investmentAnalysis.StockAnalysis) == 0 && len(investmentAnalysis.BondAnalysis) == 0) {
		app.failedValidationResponse(w, r, map[string]string{"investment_analysis": "no investments to analyze"})
		return
	}

	profile := UserPortfolioProfile{
		UserTimeHorizon:    user.TimeHorizon,
		UserRiskTolerance:  user.RiskTolerance,
		InvestmentGoals:    goals,
		InvestmentAnalysis: *investmentAnalysis,
	}
	body, err := app.renderInvestmentPortfolioPrompt(profile)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	setSSEHeaders(w.Header())
	w.WriteHeader(http.StatusOK)
	fl.Flush()

	res, err := app.streamLLMToSSE(
		r.Context(),
		w, fl,
		app.config.api.apikeys.groq.url,
		map[string]string{"Authorization": "Bearer " + app.config.api.apikeys.groq.key},
		body,
	)
	if err != nil {
		log.Error("portfolio analysis stream failed",
			zap.Error(err),
			zap.Int("partial_bytes", len(res.Text)),
		)
		_ = writeSSEEventJSON(w, fl, "error", map[string]string{
			"error": classifyLLMStreamError(err),
		})
		return
	}

	header, llmAnalysis, footer, parseErr := parseLLMResponse(res.Text)
	if parseErr != nil {
		log.Error("portfolio analysis stream parse failed",
			zap.Error(parseErr),
			zap.Int("response_bytes", len(res.Text)),
		)
		_ = writeSSEEventJSON(w, fl, "error", map[string]string{
			"error": "failed to parse model output",
		})
		return
	}

	analyzed := &data.LLMAnalyzedPortfolio{
		Header:   header,
		Analysis: llmAnalysis,
		Footer:   footer,
	}

	// Persistence is best-effort. The user already has the streamed
	// answer in their browser; a DB write failure should not turn a
	// successful analysis into a visible error. The downstream
	// /analysis/summary endpoint reads from this same table, so a
	// missed persist means the user has to re-run the analysis to
	// see it in their history — surface it loudly in logs but keep
	// the stream completion clean.
	if err := app.models.InvestmentPortfolioManager.CreateLLMAnalysisResponse(r.Context(), user.ID, analyzed); err != nil {
		log.Error("portfolio analysis stream persist failed", zap.Error(err))
	}

	_ = writeSSEEventJSON(w, fl, "done", struct {
		FinishReason string                     `json:"finish_reason,omitempty"`
		Usage        *LLMUsage                  `json:"usage,omitempty"`
		Analyzed     *data.LLMAnalyzedPortfolio `json:"analyzed"`
	}{
		FinishReason: res.FinishReason,
		Usage:        res.Usage,
		Analyzed:     analyzed,
	})

	log.Info("portfolio analysis stream completed",
		zap.Int("response_bytes", len(res.Text)),
		zap.String("finish_reason", res.FinishReason),
	)
}
