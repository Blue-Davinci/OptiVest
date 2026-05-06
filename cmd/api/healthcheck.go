package main

import (
	"net/http"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

// ---------------------------------------------------------------------------
// Liveness probe.
//
// /healthcheck is mounted on the BASE router (alongside /metrics and
// /debug/vars), deliberately outside the global middleware chain. Load
// balancers, k8s liveness probes, and Docker HEALTHCHECK directives all
// poll this endpoint at high frequency (every 1-30s); putting it through
// logRequests + the per-IP rate limiter would generate thousands of log
// lines per day per probe source and could plausibly trip the limiter on
// a misconfigured cluster.
//
// The handler is intentionally minimal:
//   - no DB ping. A liveness probe answers "is the process running?" not
//     "is every dependency healthy?"; readiness is the right place for
//     deeper checks but we do not have a separate readiness endpoint yet.
//     If you need one, copy this file and add the ping; do not bolt it
//     onto liveness.
//   - constant-time JSON output. No allocation in the hot path beyond what
//     writeJSON does.
//   - exported uptime measured from a package-level start timestamp set in
//     init(), not from request time. Operators routinely correlate uptime
//     with deployment events, so a monotonic value tied to process start
//     is what they want.
// ---------------------------------------------------------------------------

// processStart records the wall-clock instant at which this binary became
// ready to serve. It is set once at package init time and never mutated.
// Reading it via UnixNano + atomic.LoadInt64 keeps the read lock-free.
var processStart atomic.Int64

func init() {
	processStart.Store(time.Now().UnixNano())
}

// healthcheckHandler answers a 200 OK with a small JSON payload describing
// the running build. Returns the same shape regardless of state because
// the only failure mode for liveness should be "the HTTP server stopped
// answering at all" — a 200 with payload is the unambiguous "I'm alive"
// signal.
func (app *application) healthcheckHandler(w http.ResponseWriter, r *http.Request) {
	uptimeNs := time.Now().UnixNano() - processStart.Load()
	body := envelope{
		"status":     "ok",
		"version":    version,
		"env":        app.config.env,
		"uptime_sec": int64(time.Duration(uptimeNs) / time.Second),
	}
	if err := app.writeJSON(w, http.StatusOK, body, nil); err != nil {
		// At this point most of the response is already on the wire; we
		// cannot upgrade to a 5xx. Log once on the request-correlated
		// logger so the operator notices.
		app.loggerFromRequest(r).Error("healthcheck: writeJSON failed", zap.Error(err))
	}
}
