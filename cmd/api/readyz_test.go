package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// readyzTestApp wires a minimal *application against the supplied
// dependencies. Either may be nil to simulate a torn-down pool. The miniredis
// instance and *sql.DB lifecycles stay owned by the caller.
func readyzTestApp(t *testing.T, db *sql.DB, rdb *redis.Client) *application {
	t.Helper()
	return &application{
		logger:  zap.NewNop(),
		ctx:     context.Background(),
		db:      db,
		RedisDB: rdb,
		config:  config{env: "development"},
	}
}

// healthyPostgres returns an *sql.DB that is wired to a real Postgres only
// if one is reachable on the standard local port; otherwise the test is
// skipped. Unit tests that require a "Postgres is up" signal without
// actually exercising the schema use this — the underlying integration
// e2e job in CI is what proves the path against a real cluster.
//
// We do not embed a Postgres test container in unit tests because it is
// expensive and the production path we care about (db.PingContext) is
// trivially exercised by the e2e job. The unit tests instead focus on the
// failure paths, which are the interesting ones for a readiness probe.
func healthyPostgres(t *testing.T) *sql.DB {
	t.Helper()
	dsn := "postgres://optivest:optivest@127.0.0.1:5432/optivest?sslmode=disable&connect_timeout=1"
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Skipf("skip: sql.Open postgres: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 750*time.Millisecond)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		t.Skipf("skip: no local postgres reachable for happy-path test: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// downPostgres returns an *sql.DB wired to a guaranteed-closed port so
// PingContext fails fast. lib/pq honors the DSN connect_timeout, but we
// also belt-and-brace with a 1s context deadline at the call site so the
// failure mode is bounded regardless of driver behavior.
func downPostgres(t *testing.T) *sql.DB {
	t.Helper()
	dsn := "postgres://x:x@127.0.0.1:1/x?sslmode=disable&connect_timeout=1"
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// healthyRedis spins up an in-process miniredis and returns a real
// go-redis client pointed at it. Cleaning up is automatic via t.Cleanup.
func healthyRedis(t *testing.T) *redis.Client {
	t.Helper()
	mr := miniredis.RunT(t)
	return redis.NewClient(&redis.Options{Addr: mr.Addr()})
}

// downRedis returns a go-redis client pointed at a port that is reserved
// by an immediately-closed listener. Any Ping against it fails inside the
// dial timeout. We deliberately do not use 127.0.0.1:1 verbatim because
// the kernel rejects connect(2) before the dial timeout fires on Linux,
// which is fine for "down" but not great for "we exercised the timeout
// path"; the explicit closed-listener gives us a TCP RST without
// involving privileged ports.
func downRedis(t *testing.T) *redis.Client {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	addr := l.Addr().String()
	_ = l.Close() // free the port immediately so subsequent dials fail
	return redis.NewClient(&redis.Options{
		Addr:        addr,
		DialTimeout: 100 * time.Millisecond,
		MaxRetries:  -1,
	})
}

// fire issues a GET to /readyz and returns the recorded response. We invoke
// the handler directly rather than through the router because the router
// wiring is exercised in TestReadyz_RouteRegistered separately, and a
// direct call gives the test a deterministic ctx.
func fire(t *testing.T, app *application) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil).
		WithContext(context.Background())
	app.readyzHandler(rec, req)
	return rec
}

// decodeReadyz parses the response body and returns its checks map and
// top-level status field. Any malformed response fails the test
// immediately because /readyz is a contract endpoint with downstream
// consumers (k8s, LBs).
func decodeReadyz(t *testing.T, rec *httptest.ResponseRecorder) (string, map[string]string) {
	t.Helper()
	var raw struct {
		Status string            `json:"status"`
		Checks map[string]string `json:"checks"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("decode body: %v\nbody: %s", err, rec.Body.String())
	}
	return raw.Status, raw.Checks
}

// TestReadyz_HappyPath asserts the 200/ready contract when both deps are
// reachable. Postgres-availability skips if no local cluster is up, so the
// test is non-flaky in cold environments while still asserting the path
// when the developer has the docker stack running.
func TestReadyz_HappyPath(t *testing.T) {
	app := readyzTestApp(t, healthyPostgres(t), healthyRedis(t))

	rec := fire(t, app)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", got)
	}
	status, checks := decodeReadyz(t, rec)
	if status != "ready" {
		t.Errorf("status = %q, want %q", status, "ready")
	}
	if checks["postgres"] != string(readyStatusOK) {
		t.Errorf("checks.postgres = %q, want %q", checks["postgres"], readyStatusOK)
	}
	if checks["redis"] != string(readyStatusOK) {
		t.Errorf("checks.redis = %q, want %q", checks["redis"], readyStatusOK)
	}
}

// TestReadyz_PostgresDown is the "Postgres is unreachable, Redis is fine"
// failure mode. The orchestrator should pull this instance out of
// rotation; we assert the 503 and the per-dep status disclosure.
func TestReadyz_PostgresDown(t *testing.T) {
	app := readyzTestApp(t, downPostgres(t), healthyRedis(t))

	rec := fire(t, app)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503; body=%s", rec.Code, rec.Body.String())
	}
	status, checks := decodeReadyz(t, rec)
	if status != "not_ready" {
		t.Errorf("status = %q, want %q", status, "not_ready")
	}
	if checks["postgres"] != string(readyStatusDown) {
		t.Errorf("checks.postgres = %q, want %q", checks["postgres"], readyStatusDown)
	}
	if checks["redis"] != string(readyStatusOK) {
		t.Errorf("checks.redis = %q, want %q", checks["redis"], readyStatusOK)
	}
}

// TestReadyz_RedisDown mirrors the Postgres-down case for Redis. The
// dependency-isolation property of /readyz means a single failing dep
// must produce a single failing check, never a cascade. Skips silently
// when no local Postgres is reachable (the same behavior as the happy
// path test); CI's e2e job covers this path against a real cluster.
func TestReadyz_RedisDown(t *testing.T) {
	t.Parallel()
	app := readyzTestApp(t, healthyPostgres(t), downRedis(t))

	rec := fire(t, app)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503; body=%s", rec.Code, rec.Body.String())
	}
	status, checks := decodeReadyz(t, rec)
	if status != "not_ready" {
		t.Errorf("status = %q, want %q", status, "not_ready")
	}
	if checks["postgres"] != string(readyStatusOK) {
		t.Errorf("checks.postgres = %q, want %q", checks["postgres"], readyStatusOK)
	}
	if checks["redis"] != string(readyStatusDown) {
		t.Errorf("checks.redis = %q, want %q", checks["redis"], readyStatusDown)
	}
}

// TestReadyz_BothDown exercises the cascading-failure case. Both pings
// fail, both report down, and the overall verdict is not_ready. This is
// the test that catches regressions in the per-dep aggregation logic
// (e.g. someone short-circuiting after the first failure and never
// running the second probe).
func TestReadyz_BothDown(t *testing.T) {
	t.Parallel()
	app := readyzTestApp(t, downPostgres(t), downRedis(t))

	rec := fire(t, app)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
	_, checks := decodeReadyz(t, rec)
	if checks["postgres"] != string(readyStatusDown) {
		t.Errorf("checks.postgres = %q, want %q", checks["postgres"], readyStatusDown)
	}
	if checks["redis"] != string(readyStatusDown) {
		t.Errorf("checks.redis = %q, want %q", checks["redis"], readyStatusDown)
	}
}

// TestReadyz_NilHandlesProduceDown covers the defense-in-depth nil-pool
// branch. In production the handles are always non-nil (main exits on
// openDB error), but a future refactor that drops the assignment must
// not be allowed to turn a probe call into a nil-pointer panic.
func TestReadyz_NilHandlesProduceDown(t *testing.T) {
	t.Parallel()
	app := readyzTestApp(t, nil, nil)

	rec := fire(t, app)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
	status, checks := decodeReadyz(t, rec)
	if status != "not_ready" {
		t.Errorf("status = %q, want %q", status, "not_ready")
	}
	if checks["postgres"] != string(readyStatusDown) {
		t.Errorf("checks.postgres = %q, want %q", checks["postgres"], readyStatusDown)
	}
	if checks["redis"] != string(readyStatusDown) {
		t.Errorf("checks.redis = %q, want %q", checks["redis"], readyStatusDown)
	}
}

// TestReadyz_BodyShape pins down the exposed JSON contract so any future
// edit that drops a top-level field fails loudly. Downstream consumers
// (operator dashboards, k8s readiness logs) parse this shape.
func TestReadyz_BodyShape(t *testing.T) {
	t.Parallel()
	app := readyzTestApp(t, downPostgres(t), downRedis(t))

	rec := fire(t, app)
	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	required := []string{"status", "version", "env", "uptime_sec", "checks"}
	for _, k := range required {
		if _, ok := body[k]; !ok {
			t.Errorf("body missing required field: %q", k)
		}
	}
	if got, ok := body["env"].(string); !ok || got != "development" {
		t.Errorf("body.env = %v, want \"development\"", body["env"])
	}
}

// TestReadyz_DoesNotLeakDriverErrors guards the don't-disclose-internals
// invariant. The body must report a coarse "down" verdict and must not
// inline raw driver error strings, which can leak hostnames and ports.
// This test is intentionally pessimistic: it checks for substrings that
// commonly appear in lib/pq and go-redis errors.
func TestReadyz_DoesNotLeakDriverErrors(t *testing.T) {
	t.Parallel()
	app := readyzTestApp(t, downPostgres(t), downRedis(t))

	rec := fire(t, app)
	body := rec.Body.String()
	for _, leaked := range []string{
		"127.0.0.1",
		"connection refused",
		"dial tcp",
		"i/o timeout",
		"sql:",
		"pq:",
	} {
		if strings.Contains(body, leaked) {
			t.Errorf("response body should not contain driver-level detail %q\nbody=%s", leaked, body)
		}
	}
}

// TestReadyz_RouteRegistered verifies /readyz is wired into the base
// router and reachable without auth, rate limiting, or logRequests in
// front of it. We do not stand up real deps here — the failure path is
// fine for confirming the route wiring; the response code only matters in
// that it is one of {200, 503}, both of which prove the handler ran.
func TestReadyz_RouteRegistered(t *testing.T) {
	t.Parallel()
	app := readyzTestApp(t, downPostgres(t), downRedis(t))
	app.config.cors.trustedOrigins = []string{}

	server := httptest.NewServer(app.routes())
	t.Cleanup(server.Close)

	resp, err := http.Get(server.URL + "/readyz")
	if err != nil {
		t.Fatalf("GET /readyz: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 200 or 503", resp.StatusCode)
	}
}
