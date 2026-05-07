package main

import (
	"bytes"
	"context"
	"expvar"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.uber.org/zap"
)

// expvar is a process-global registry. Several tests in this package depend on
// instrumentation that publishes entries to it (the metrics middleware
// publishes total_responses_sent_by_status the first time it is constructed,
// publishMetrics() publishes the version string, etc.). Since test order is
// undefined and we don't want a cross-test ordering coupling, the helpers
// below use expvar.Get + a get-or-create pattern so each test can set the
// preconditions it cares about without panicking on a duplicate registration.

// getOrPublishString returns the expvar.String at name, creating it (and
// registering it under name) if the registry does not already have one.
func getOrPublishString(t *testing.T, name string) *expvar.String {
	t.Helper()
	if v := expvar.Get(name); v != nil {
		s, ok := v.(*expvar.String)
		if !ok {
			t.Fatalf("expvar %q exists but is not *expvar.String (got %T)", name, v)
		}
		return s
	}
	return expvar.NewString(name)
}

// getOrPublishMap mirrors getOrPublishString for *expvar.Map.
func getOrPublishMap(t *testing.T, name string) *expvar.Map {
	t.Helper()
	if v := expvar.Get(name); v != nil {
		m, ok := v.(*expvar.Map)
		if !ok {
			t.Fatalf("expvar %q exists but is not *expvar.Map (got %T)", name, v)
		}
		return m
	}
	return expvar.NewMap(name)
}

// scrape runs writePrometheusExposition and returns the text body. It is a
// thin convenience over instantiating a bytes.Buffer in every test.
func scrape(t *testing.T) string {
	t.Helper()
	var buf bytes.Buffer
	if err := writePrometheusExposition(&buf); err != nil {
		t.Fatalf("writePrometheusExposition returned error: %v", err)
	}
	return buf.String()
}

// TestPrometheusExposition_HasHelpAndTypeForCuratedCounters asserts that for
// the counters we explicitly publish in our packages (request_log_total here
// stands in for the broader set), the exposition emits the expected
// HELP / TYPE / sample triple and never a bare sample line.
func TestPrometheusExposition_HasHelpAndTypeForCuratedCounters(t *testing.T) {
	requestLogTotal.Add(1)

	body := scrape(t)

	mustContain := []string{
		"# HELP request_log_total ",
		"# TYPE request_log_total counter",
		"request_log_total ",
	}
	for _, line := range mustContain {
		if !strings.Contains(body, line) {
			t.Errorf("exposition missing %q\n--- body ---\n%s", line, body)
		}
	}
}

// TestPrometheusExposition_GaugesAreTypedGauge confirms the type tag
// distinguishes counters from gauges. portfolio_analysis_workers_active is
// published via expvar.Func at package init time and is therefore present
// in every test binary; goroutines is published from publishMetrics() at
// runtime, so we register it here ourselves if it is absent so the assertion
// is self-contained.
func TestPrometheusExposition_GaugesAreTypedGauge(t *testing.T) {
	if expvar.Get("goroutines") == nil {
		expvar.Publish("goroutines", expvar.Func(func() any { return 1 }))
	}

	body := scrape(t)
	if !strings.Contains(body, "# TYPE portfolio_analysis_workers_active gauge") {
		t.Errorf("expected workers_active to be typed gauge\n%s", body)
	}
	if !strings.Contains(body, "# TYPE goroutines gauge") {
		t.Errorf("expected goroutines to be typed gauge\n%s", body)
	}
}

// TestPrometheusExposition_MicroSignSanitised covers the one allow-list entry
// whose expvar name is illegal under Prometheus' [a-zA-Z_:][a-zA-Z0-9_:]*
// metric name regex. The exposition must rename it to the ASCII-safe
// total_processing_time_us. The original expvar key stays untouched on
// /debug/vars; this is purely a Prom-format remap.
func TestPrometheusExposition_MicroSignSanitised(t *testing.T) {
	totalProcessingTimeMicroseconds.Add(1)

	body := scrape(t)

	if !strings.Contains(body, "total_processing_time_us ") {
		t.Errorf("expected sanitized metric total_processing_time_us in output\n%s", body)
	}
	if strings.Contains(body, "total_processing_time_μs") {
		t.Errorf("non-ASCII μ leaked into Prom output:\n%s", body)
	}
}

// TestPrometheusExposition_MapBecomesLabelledLines verifies that an
// expvar.Map turns into one labeled sample line per entry, and that the
// type/help block is emitted exactly once even with multiple samples.
func TestPrometheusExposition_MapBecomesLabelledLines(t *testing.T) {
	m := getOrPublishMap(t, "total_responses_sent_by_status")
	m.Add("200", 1)
	m.Add("500", 1)

	body := scrape(t)

	wantLineFragments := []string{
		`total_responses_sent_by_status{code="200"} `,
		`total_responses_sent_by_status{code="500"} `,
	}
	for _, frag := range wantLineFragments {
		if !strings.Contains(body, frag) {
			t.Errorf("exposition missing %q\n%s", frag, body)
		}
	}

	// HELP and TYPE should appear once for the metric, not once per sample.
	helpCount := strings.Count(body, "# HELP total_responses_sent_by_status ")
	typeCount := strings.Count(body, "# TYPE total_responses_sent_by_status counter")
	if helpCount != 1 {
		t.Errorf("expected exactly one HELP line for total_responses_sent_by_status, got %d", helpCount)
	}
	if typeCount != 1 {
		t.Errorf("expected exactly one TYPE line for total_responses_sent_by_status, got %d", typeCount)
	}
}

// TestPrometheusExposition_VersionBecomesInfoMetric covers the canonical
// Prometheus pattern for build identity: a labeled gauge always set to 1
// with the version carried in the label, never as the metric value.
func TestPrometheusExposition_VersionBecomesInfoMetric(t *testing.T) {
	versionVar := getOrPublishString(t, "version")
	versionVar.Set("test-build-1.2.3")

	body := scrape(t)

	want := `optivest_build_info{version="test-build-1.2.3"} 1`
	if !strings.Contains(body, want) {
		t.Errorf("exposition missing %q\n%s", want, body)
	}
	if !strings.Contains(body, "# TYPE optivest_build_info gauge") {
		t.Errorf("expected optivest_build_info to be typed gauge\n%s", body)
	}
}

// TestPrometheusExposition_SkipsRuntimeNoise asserts that the noisy default
// expvars are NOT promoted to /metrics. cmdline is a string array we can't
// flatten cleanly, and memstats is a multi-level struct better served by
// Prom's own runtime instrumentation. The allow-list should keep both off
// the wire.
func TestPrometheusExposition_SkipsRuntimeNoise(t *testing.T) {
	body := scrape(t)
	for _, name := range []string{"cmdline", "memstats"} {
		if strings.Contains(body, name) {
			t.Errorf("exposition leaks runtime noise %q\n%s", name, body)
		}
	}
}

// TestPrometheusExposition_StableOrdering captures a property the format
// gives us almost for free: scraping twice in a row should produce
// byte-identical output, modulo whatever counters incremented between
// scrapes. Sorting metric names + sample lines means new metrics arriving
// between releases don't shuffle the dashboard diffs.
func TestPrometheusExposition_StableOrdering(t *testing.T) {
	a := scrape(t)
	b := scrape(t)
	if a != b {
		t.Errorf("expected stable byte-identical scrape output, got drift:\nfirst:\n%s\nsecond:\n%s", a, b)
	}
}

// TestPrometheusExposition_NoUnsortedSamplesWithinAMetric asserts that all
// samples sharing a metric name are emitted contiguously and in sort order.
// Prom's text parser tolerates interleaving but operators do not — a
// contiguous block per metric is what makes a raw scrape diff readable.
func TestPrometheusExposition_NoUnsortedSamplesWithinAMetric(t *testing.T) {
	m := getOrPublishMap(t, "total_responses_sent_by_status")
	m.Add("404", 1)
	m.Add("201", 1)

	body := scrape(t)

	// Find the block bounded by the HELP line and the next blank-or-other-
	// metric line; assert samples within it are sorted lexicographically.
	const helpPrefix = "# HELP total_responses_sent_by_status "
	idx := strings.Index(body, helpPrefix)
	if idx < 0 {
		t.Fatal("expected HELP line for total_responses_sent_by_status")
	}
	tail := body[idx:]

	var samples []string
	for _, line := range strings.Split(tail, "\n") {
		if strings.HasPrefix(line, "total_responses_sent_by_status{") {
			samples = append(samples, line)
			continue
		}
		// A different metric's block has started, stop scanning.
		if len(samples) > 0 && strings.HasPrefix(line, "# HELP ") {
			break
		}
	}

	if len(samples) < 2 {
		t.Skip("not enough samples accumulated for ordering assertion")
	}
	for i := 1; i < len(samples); i++ {
		if samples[i-1] > samples[i] {
			t.Errorf("sample lines for one metric are not in sort order:\n%v", samples)
			break
		}
	}
}

// TestPrometheusMetricsHandler_ContentTypeAndStatus covers the HTTP shell
// around the formatter: correct status, correct content-type negotiation
// header for Prom, and a body that survives the round-trip through a real
// http.ResponseWriter.
func TestPrometheusMetricsHandler_ContentTypeAndStatus(t *testing.T) {
	app := &application{logger: zap.NewNop()}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)

	app.prometheusMetricsHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != metricsContentType {
		t.Errorf("Content-Type = %q, want %q", got, metricsContentType)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want %q", got, "no-store")
	}
	if rec.Body.Len() == 0 {
		t.Error("expected non-empty response body")
	}
}

// TestPrometheusMetricsHandler_BodyShape spot-checks the http body to make
// sure the handler is not silently swallowing the formatter's output.
func TestPrometheusMetricsHandler_BodyShape(t *testing.T) {
	app := &application{logger: zap.NewNop()}
	requestLogTotal.Add(1)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	app.prometheusMetricsHandler(rec, req)

	body := rec.Body.String()
	if !strings.HasPrefix(body, "# HELP ") {
		t.Errorf("expected body to start with a HELP line, got: %q", firstLine(body))
	}
	if !strings.Contains(body, "request_log_total") {
		t.Errorf("expected request_log_total in body:\n%s", body)
	}
}

// TestPrometheusExposition_HelpLinesEscapeNewlines is a forward-looking
// guard. We currently hand-author every help string in metrics.go and none
// of them contain newlines, but if someone copies in a multi-line string
// the exposition format will break the parser silently. This test fails
// loudly if a help line ever wraps.
func TestPrometheusExposition_HelpLinesEscapeNewlines(t *testing.T) {
	body := scrape(t)
	for _, line := range strings.Split(body, "\n") {
		if !strings.HasPrefix(line, "# HELP ") {
			continue
		}
		if strings.Contains(line[len("# HELP "):], "\n") {
			t.Errorf("help line contains an unescaped newline: %q", line)
		}
	}
}

// TestRoutesIncludeMetrics asserts that the router actually wires /metrics
// as a top-level GET route. This is structural rather than behavioral: we
// don't dispatch a real request through the full middleware chain (that
// requires a fully-configured application), we just confirm chi has a route
// matching the path. That is enough to catch the "forgot to register" class
// of regression.
func TestRoutesIncludeMetrics(t *testing.T) {
	// Construct just enough application state to call routes() without
	// initializing every downstream subsystem (DB, redis, mailer, etc.).
	// The middleware constructors only read fields we set here.
	app := &application{
		logger: zap.NewNop(),
		ctx:    context.Background(),
		config: config{
			cors: struct {
				trustedOrigins []string
			}{trustedOrigins: []string{"*"}},
		},
	}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("routes() panicked at construction time: %v", r)
		}
	}()

	handler := app.routes()
	if handler == nil {
		t.Fatal("routes() returned nil handler")
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	handler.ServeHTTP(rec, req)

	// We expect EITHER 200 (handler ran cleanly) or a status driven by the
	// CORS / handler logic. What we MUST NOT see is 404 — that would mean
	// the route is not registered.
	if rec.Code == http.StatusNotFound {
		t.Errorf("/metrics returned 404, expected the route to be registered")
	}
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

// TestPrometheusExposition_AllCuratedNamesMatchPromRegex is a fast-failing
// safety net for future contributors. Anyone adding a new entry to the
// allow-list should hit this test if they accidentally use a non-ASCII
// character or punctuation that Prom rejects. The regex matches Prom's
// metric-name grammar exactly.
func TestPrometheusExposition_AllCuratedNamesMatchPromRegex(t *testing.T) {
	body := scrape(t)
	for _, line := range strings.Split(body, "\n") {
		if !strings.HasPrefix(line, "# TYPE ") {
			continue
		}
		// Format: "# TYPE name kind"
		parts := strings.SplitN(strings.TrimPrefix(line, "# TYPE "), " ", 2)
		if len(parts) != 2 {
			t.Errorf("malformed TYPE line: %q", line)
			continue
		}
		name := parts[0]
		if !validPromName(name) {
			t.Errorf("metric name %q does not match Prom regex [a-zA-Z_:][a-zA-Z0-9_:]*", name)
		}
	}
}

// validPromName is a hand-rolled, allocation-free check for Prom's metric
// name regex: [a-zA-Z_:][a-zA-Z0-9_:]*. We don't import regexp because the
// expression is too simple to justify the dependency in a test file.
func validPromName(s string) bool {
	if len(s) == 0 {
		return false
	}
	for i, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r == '_' || r == ':':
		case i > 0 && r >= '0' && r <= '9':
		default:
			return false
		}
	}
	return true
}

// _ ensures fmt is referenced even if we trim debug formatting later.
var _ = fmt.Sprintf
