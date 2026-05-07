package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"
)

// minimalApp returns the smallest possible *application that buildAPIServer
// and buildSSEServer need: a logger, a context, and the port fields read by
// the constructors. Routes are stubbed because we never actually serve a
// request through them in these tests — we test the http.Server's connection
// behavior, not the application's handler logic.
func minimalApp(t *testing.T, ctx context.Context) *application {
	t.Helper()
	return &application{
		logger: zap.NewNop(),
		ctx:    ctx,
		config: config{
			port: 0,
		},
	}
}

// TestBuildAPIServer_Hardening asserts that the server-construction helper
// produces an http.Server with every P0/P1 hardening field populated. This
// catches regressions where someone removes a field thinking it is unused.
func TestBuildAPIServer_Hardening(t *testing.T) {
	app := minimalApp(t, context.Background())
	// Use apiServerConfig directly with a stub handler so we don't pull in
	// app.routes() (which needs the expvar metric publishers).
	srv := app.apiServerConfig(http.NewServeMux())

	if srv.ReadHeaderTimeout != apiReadHeaderTimeout {
		t.Errorf("ReadHeaderTimeout = %v, want %v", srv.ReadHeaderTimeout, apiReadHeaderTimeout)
	}
	if srv.ReadTimeout != apiReadTimeout {
		t.Errorf("ReadTimeout = %v, want %v", srv.ReadTimeout, apiReadTimeout)
	}
	if srv.WriteTimeout != apiWriteTimeout {
		t.Errorf("WriteTimeout = %v, want %v", srv.WriteTimeout, apiWriteTimeout)
	}
	if srv.IdleTimeout != apiIdleTimeout {
		t.Errorf("IdleTimeout = %v, want %v", srv.IdleTimeout, apiIdleTimeout)
	}
	if srv.ErrorLog == nil {
		t.Error("ErrorLog must be set so http.Server panics route through zap")
	}
	if srv.BaseContext == nil {
		t.Error("BaseContext must be set so shutdown propagates to handlers")
	}
	if srv.ConnContext == nil {
		t.Error("ConnContext must be set so connections get connID stamps")
	}

	// BaseContext should return the application context.
	got := srv.BaseContext(nil)
	if got != app.ctx {
		t.Error("BaseContext must return the application's lifecycle context")
	}
}

// TestBuildSSEServer_Hardening mirrors the API check but allows the SSE
// server's intentionally relaxed write/idle timeouts.
func TestBuildSSEServer_Hardening(t *testing.T) {
	app := minimalApp(t, context.Background())
	srv := app.sseServerConfig(http.NewServeMux())

	if srv.ReadHeaderTimeout != sseReadHeaderTimeout {
		t.Errorf("ReadHeaderTimeout = %v, want %v", srv.ReadHeaderTimeout, sseReadHeaderTimeout)
	}
	if srv.WriteTimeout != sseWriteTimeout {
		t.Errorf("WriteTimeout = %v, want %v (SSE streams must not have a write deadline)", srv.WriteTimeout, sseWriteTimeout)
	}
	if srv.ErrorLog == nil {
		t.Error("ErrorLog must be set on the SSE server too")
	}
	if srv.BaseContext == nil {
		t.Error("BaseContext must be set on the SSE server too")
	}
	if srv.ConnContext == nil {
		t.Error("ConnContext must be set on the SSE server too")
	}
}

// TestConnContext_StampsMonotonicID verifies that ConnContext attaches a
// non-zero, monotonically-increasing ID into the per-connection context. We
// can't easily snapshot the counter atomically across two calls (other tests
// in this binary may also invoke connContext), so we only assert (a) values
// are non-zero and (b) the second call returns a strictly larger value.
func TestConnContext_StampsMonotonicID(t *testing.T) {
	a := connContext(context.Background(), nil)
	b := connContext(context.Background(), nil)

	idA := contextConnID(a)
	idB := contextConnID(b)

	if idA == 0 || idB == 0 {
		t.Fatalf("expected non-zero connection IDs, got idA=%d idB=%d", idA, idB)
	}
	if idB <= idA {
		t.Errorf("expected monotonically-increasing connection IDs, got idA=%d idB=%d", idA, idB)
	}
}

// TestAPIServer_ReadHeaderTimeout exercises the actual timeout against a real
// loopback listener. It opens a TCP connection, sends only a partial HTTP
// request line (no terminating CRLF on headers), and asserts the server closes
// the connection within the configured ReadHeaderTimeout window. This is the
// canonical Slowloris probe.
//
// Without ReadHeaderTimeout the test would hang until the test framework
// killed it; with the timeout we expect the connection to drop in <2x the
// configured budget.
func TestAPIServer_ReadHeaderTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("real-network timeout test; skipped under -short")
	}

	// Bind to an ephemeral port; we never call ListenAndServe directly because
	// we want to control teardown precisely.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to bind loopback listener: %v", err)
	}
	defer ln.Close()

	app := minimalApp(t, context.Background())
	srv := app.apiServerConfig(http.NewServeMux()) // never reached
	// Override to a tight value so the test runs quickly.
	srv.ReadHeaderTimeout = 250 * time.Millisecond

	go func() {
		_ = srv.Serve(ln)
	}()
	defer func() {
		_ = srv.Shutdown(context.Background())
	}()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("failed to dial server: %v", err)
	}
	defer conn.Close()

	// Send only the request line + Host header; deliberately omit the blank
	// line that signals end-of-headers.
	if _, err := fmt.Fprint(conn, "GET / HTTP/1.1\r\nHost: example.com\r\n"); err != nil {
		t.Fatalf("failed to write partial request: %v", err)
	}

	// We expect the server to close the connection within ~ReadHeaderTimeout.
	// Allow generous slack for slow CI machines.
	if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("failed to set read deadline: %v", err)
	}

	start := time.Now()
	br := bufio.NewReader(conn)
	_, readErr := br.ReadString('\n')
	elapsed := time.Since(start)

	switch {
	case readErr == nil:
		t.Fatal("expected server to close the connection without sending a response")
	case errors.Is(readErr, io.EOF) || strings.Contains(readErr.Error(), "EOF") || strings.Contains(readErr.Error(), "connection reset"):
		// Expected: server gave up on the headers and closed the TCP conn.
	case errors.Is(readErr, io.ErrUnexpectedEOF):
		// Same outcome, different error wrapping in some Go versions.
	default:
		t.Fatalf("unexpected read error from server: %v", readErr)
	}

	if elapsed > time.Second {
		t.Errorf("server took %v to enforce ReadHeaderTimeout (expected ~250ms)", elapsed)
	}
}
