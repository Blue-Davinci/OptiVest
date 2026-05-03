package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// rateLimitTestApp returns a minimal *application wired to either a real
// in-memory miniredis-backed client (mr != nil) or a misconfigured client
// pointing at a closed listener (mr == nil) so the limiter Allow() call
// fails. The rate limiter middleware is the System Under Test; we don't
// route through any real handler.
func rateLimitTestApp(t *testing.T, mr *miniredis.Miniredis, rps float64, burst int, enabled bool) (*application, *redis.Client) {
	t.Helper()

	var rdb *redis.Client
	if mr != nil {
		rdb = redis.NewClient(&redis.Options{Addr: mr.Addr()})
	} else {
		// 127.0.0.1:1 is a guaranteed-closed port; any Allow() against this
		// client returns an immediate error, exercising the fail-open path.
		rdb = redis.NewClient(&redis.Options{
			Addr:        "127.0.0.1:1",
			DialTimeout: 50 * time.Millisecond,
			MaxRetries:  -1,
		})
	}

	app := &application{
		logger:  zap.NewNop(),
		ctx:     context.Background(),
		RedisDB: rdb,
		config: config{
			limiter: struct {
				rps     float64
				burst   int
				enabled bool
			}{
				rps:     rps,
				burst:   burst,
				enabled: enabled,
			},
		},
	}
	return app, rdb
}

// drive issues a GET to the rate-limit middleware wrapped around a handler
// that just returns 200 OK with a known body. Returns the response so tests
// can inspect status, headers, and counters in the middleware-recorded path.
func drive(t *testing.T, app *application) *http.Response {
	t.Helper()
	mw := app.rateLimit(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "203.0.113.7:9999" // a fixed, RFC5737-documentation IP
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)
	return rec.Result()
}

// TestRateLimit_AllowsUnderThreshold verifies that requests issued within the
// configured burst budget are all allowed and carry the documentation
// headers (X-RateLimit-Limit / -Remaining) so well-behaved clients can
// self-throttle.
func TestRateLimit_AllowsUnderThreshold(t *testing.T) {
	mr := miniredis.RunT(t)
	app, _ := rateLimitTestApp(t, mr, 10, 5, true)

	for i := 0; i < 5; i++ {
		resp := drive(t, app)
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("request %d: status = %d, want 200 (under burst)", i+1, resp.StatusCode)
		}
		if got := resp.Header.Get("X-RateLimit-Limit"); got != "5" {
			t.Errorf("request %d: X-RateLimit-Limit = %q, want %q", i+1, got, "5")
		}
		if resp.Header.Get("X-RateLimit-Remaining") == "" {
			t.Errorf("request %d: missing X-RateLimit-Remaining header", i+1)
		}
	}
}

// TestRateLimit_DeniesOverThreshold verifies that once the burst is
// exhausted, the next request gets a 429 and a populated Retry-After.
func TestRateLimit_DeniesOverThreshold(t *testing.T) {
	mr := miniredis.RunT(t)
	app, _ := rateLimitTestApp(t, mr, 1, 2, true) // 1rps, burst 2

	// Burn through the burst.
	for i := 0; i < 2; i++ {
		resp := drive(t, app)
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("priming request %d: status = %d, want 200", i+1, resp.StatusCode)
		}
	}

	// The next request must be 429.
	resp := drive(t, app)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("over-burst request: status = %d, want %d", resp.StatusCode, http.StatusTooManyRequests)
	}
	if ra := resp.Header.Get("Retry-After"); ra == "" {
		t.Error("Retry-After header missing on 429")
	} else if n, err := strconv.Atoi(ra); err != nil || n < 1 {
		t.Errorf("Retry-After = %q, want a positive integer (RFC 7231 delta-seconds)", ra)
	}
	if reset := resp.Header.Get("X-RateLimit-Reset"); reset == "" {
		t.Error("X-RateLimit-Reset header missing on 429")
	}
}

// TestRateLimit_DisabledShortCircuits asserts that enabled=false skips Redis
// entirely. We confirm by passing a client wired to a closed port — if the
// disable check were missing, that would surface as fail-open + warning, but
// here we expect a clean pass with no Redis touched at all.
func TestRateLimit_DisabledShortCircuits(t *testing.T) {
	app, _ := rateLimitTestApp(t, nil, 1, 1, false)

	resp := drive(t, app)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("disabled limiter: status = %d, want 200", resp.StatusCode)
	}
	if resp.Header.Get("X-RateLimit-Limit") != "" {
		t.Error("disabled limiter must not emit X-RateLimit-Limit headers")
	}
}

// TestRateLimit_FailOpenOnRedisError is the most important test in this file:
// when Redis is unreachable, the middleware must allow the request through
// (fail-open) rather than 503-ing the entire API. Wiring the client to a
// guaranteed-dead port causes Allow() to return an error.
func TestRateLimit_FailOpenOnRedisError(t *testing.T) {
	app, _ := rateLimitTestApp(t, nil, 100, 100, true)

	// First, prove Redis really is unreachable so the test is testing what
	// it thinks it's testing. If 127.0.0.1:1 ever becomes reachable in some
	// future CI environment the test must still pass, but the assertion
	// below would no longer be exercising the fail-open path — surface it.
	probeCtx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	if err := app.RedisDB.Ping(probeCtx).Err(); err == nil {
		t.Fatal("test setup invariant broken: 127.0.0.1:1 is reachable, fail-open path not exercised")
	}

	resp := drive(t, app)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Redis-down request: status = %d, want 200 (fail-open)", resp.StatusCode)
	}
}

// TestRateLimit_PerIPIsolation confirms two distinct IPs maintain
// independent buckets — exhausting one IP's budget must not affect another.
// This is the property that prevents a noisy neighbour from rate-limiting
// every other client behind the same load balancer.
func TestRateLimit_PerIPIsolation(t *testing.T) {
	mr := miniredis.RunT(t)
	app, _ := rateLimitTestApp(t, mr, 1, 1, true)

	driveAs := func(ip string) *http.Response {
		mw := app.rateLimit(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = ip + ":50000"
		rec := httptest.NewRecorder()
		mw.ServeHTTP(rec, req)
		return rec.Result()
	}

	// Exhaust IP A's bucket (rps=1, burst=1 → second request hits 429).
	r1 := driveAs("198.51.100.1")
	if r1.StatusCode != http.StatusOK {
		t.Fatalf("A first: %d", r1.StatusCode)
	}
	r1.Body.Close()

	r2 := driveAs("198.51.100.1")
	if r2.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("A second: %d, want 429", r2.StatusCode)
	}
	r2.Body.Close()

	// IP B starts fresh and should be allowed.
	r3 := driveAs("198.51.100.2")
	if r3.StatusCode != http.StatusOK {
		t.Fatalf("B first: %d (IP isolation broken — A's exhausted bucket affected B)", r3.StatusCode)
	}
	r3.Body.Close()
}
