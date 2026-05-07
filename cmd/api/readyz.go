package main

import (
	"context"
	"database/sql"
	"net/http"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// ---------------------------------------------------------------------------
// Readiness probe.
//
// /readyz is mounted on the BASE router (alongside /healthcheck, /metrics,
// /debug/vars), deliberately outside the global middleware chain. The same
// reasoning that applies to /healthcheck applies here, doubly so:
//
//   - high-frequency probe traffic (k8s readinessProbe / AWS target group
//     health check, both at single-digit-second intervals) would drown the
//     real request log if it went through logRequests.
//   - per-IP rate limiting on the readiness endpoint is exactly the wrong
//     thing: the LB would be told the instance is unready precisely when
//     the LB starts probing harder, the classic feedback loop.
//
// What /readyz answers vs /healthcheck:
//
//   /healthcheck — liveness. "Is the process running?" Always 200 unless the
//                  HTTP server itself stopped serving. Wired to k8s
//                  livenessProbe / Docker HEALTHCHECK; failure causes a
//                  container RESTART.
//   /readyz      — readiness. "Can this instance serve real traffic right
//                  now?" 200 when Postgres + Redis are reachable, 503 when
//                  any required dependency is down. Wired to k8s
//                  readinessProbe / load-balancer health checks; failure
//                  causes the instance to be REMOVED FROM ROTATION (no
//                  restart, no traffic routed in).
//
// The split is the orchestrator-pattern that prevents a brief Postgres
// hiccup from triggering a global restart cascade across every API pod.
//
// Scope is intentionally narrow: only the dependencies the request path
// actually needs to succeed.
//
//   - Postgres pool: every authenticated handler hits it.
//   - Redis: rate limiter, cache layer, notification pubsub.
//
// We deliberately do NOT check upstream third-party APIs (Alpha Vantage,
// FRED, FMP, SambaNova). Their availability is asynchronous to ours;
// reflecting their state in /readyz would let a vendor outage drain the
// pool and cause a self-inflicted outage. The application falls back
// gracefully on those vendors at the per-request layer.
//
// Body shape is intentionally minimal. /readyz is unauthenticated by
// design — k8s and load balancers do not carry tokens. We disclose dep
// *names* (postgres, redis) which are inferable from any job description,
// but we do NOT expose error strings on the wire because Go network errors
// routinely leak internal hostnames, ports, and driver-level details.
// Operators read the structured logs for the "why"; the probe just tells
// the orchestrator the "what".
// ---------------------------------------------------------------------------

// readinessTimeout bounds each per-dependency check. A hung Postgres or
// Redis must not be able to make /readyz hang past this; the LB needs a
// definitive answer. 1.5s is comfortably above realistic ping latency
// (single-digit ms on a healthy stack) and well under any sensible LB
// probe timeout (typically 5-10s).
const readinessTimeout = 1500 * time.Millisecond

// readyStatus is the per-dep verdict reported in the response body.
type readyStatus string

const (
	readyStatusOK   readyStatus = "ok"
	readyStatusDown readyStatus = "down"
)

// readyzHandler runs Postgres and Redis pings in parallel, each with an
// independent deadline, and returns 200 if both succeed or 503 if either
// fails. Failures are logged on the request-correlated logger so operators
// can correlate a 503 here with the underlying driver error.
func (app *application) readyzHandler(w http.ResponseWriter, r *http.Request) {
	checks := app.runReadinessChecks(r.Context())

	allOK := true
	for _, status := range checks {
		if status != readyStatusOK {
			allOK = false
			break
		}
	}

	uptimeNs := time.Now().UnixNano() - processStart.Load()
	body := envelope{
		"status":     readinessSummary(allOK),
		"version":    version,
		"env":        app.config.env,
		"uptime_sec": int64(time.Duration(uptimeNs) / time.Second),
		"checks":     checks,
	}

	statusCode := http.StatusOK
	if !allOK {
		statusCode = http.StatusServiceUnavailable
	}

	if err := app.writeJSON(w, statusCode, body, nil); err != nil {
		app.loggerFromRequest(r).Error("readyz: writeJSON failed", zap.Error(err))
	}
}

// readinessSummary returns the user-facing status string. Kept as a tiny
// helper so the two call sites (handler + tests) stay in lockstep.
func readinessSummary(allOK bool) string {
	if allOK {
		return "ready"
	}
	return "not_ready"
}

// runReadinessChecks fires the dependency checks concurrently, each with
// its own bounded context, and returns a stable map keyed by dep name. The
// parent context is honored: a client that cancels mid-probe (rare, but it
// happens with aggressive LB timeouts) propagates the cancel into the
// driver-level pings.
func (app *application) runReadinessChecks(parent context.Context) map[string]readyStatus {
	checks := map[string]readyStatus{
		"postgres": readyStatusDown,
		"redis":    readyStatusDown,
	}

	var (
		mu sync.Mutex
		wg sync.WaitGroup
	)
	record := func(name string, err error) {
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			checks[name] = readyStatusDown
			app.logger.Warn("readyz: dependency unhealthy",
				zap.String("dependency", name),
				zap.Error(err),
			)
			return
		}
		checks[name] = readyStatusOK
	}

	wg.Add(2)
	go func() {
		defer wg.Done()
		ctx, cancel := context.WithTimeout(parent, readinessTimeout)
		defer cancel()
		record("postgres", pingPostgres(ctx, app.db))
	}()
	go func() {
		defer wg.Done()
		ctx, cancel := context.WithTimeout(parent, readinessTimeout)
		defer cancel()
		record("redis", pingRedis(ctx, app.RedisDB))
	}()
	wg.Wait()

	return checks
}

// pingPostgres is a thin wrapper so tests can exercise the nil-safety
// branch without spinning up a real *sql.DB. In production the field is
// always non-nil (main() exits on openDB error), but defending against nil
// here turns a would-be panic into a clean 503 if the pool is somehow
// torn down.
func pingPostgres(ctx context.Context, db *sql.DB) error {
	if db == nil {
		return errReadinessNoPool
	}
	return db.PingContext(ctx)
}

// pingRedis mirrors pingPostgres for the Redis client.
func pingRedis(ctx context.Context, rdb *redis.Client) error {
	if rdb == nil {
		return errReadinessNoPool
	}
	return rdb.Ping(ctx).Err()
}

// errReadinessNoPool is returned when the dependency handle is nil, which
// in practice can only happen in tests or in a half-initialized
// application. Treating it as "down" rather than panicking keeps the probe
// honest.
var errReadinessNoPool = readinessError("dependency handle is nil")

type readinessError string

func (e readinessError) Error() string { return string(e) }
