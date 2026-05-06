package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.uber.org/zap"
)

// TestHealthcheck_Shape covers the contract a load balancer or k8s
// liveness probe expects: 200 status, JSON body with a `status` field set
// to "ok", and the version + env tags so a multi-cluster operator can tell
// at a glance which build is answering.
func TestHealthcheck_Shape(t *testing.T) {
	app := &application{
		logger: zap.NewNop(),
		config: config{env: "development"},
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthcheck", nil)
	app.healthcheckHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", got)
	}

	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("response body is not valid JSON: %v\nbody: %s", err, rec.Body.String())
	}

	if got, ok := body["status"].(string); !ok || got != "ok" {
		t.Errorf("body.status = %v, want \"ok\"", body["status"])
	}
	if _, ok := body["version"]; !ok {
		t.Error("body missing required field: version")
	}
	if got, ok := body["env"].(string); !ok || got != "development" {
		t.Errorf("body.env = %v, want \"development\"", body["env"])
	}
	if _, ok := body["uptime_sec"]; !ok {
		t.Error("body missing required field: uptime_sec")
	}
}

// TestHealthcheck_NoAuthRequired confirms that the handler is callable
// without a request context that has been through the auth middleware.
// It would be surprising for an LB probe to need an API key, and any
// future refactor that accidentally adds an auth dependency should fail
// here.
func TestHealthcheck_NoAuthRequired(t *testing.T) {
	app := &application{
		logger: zap.NewNop(),
		config: config{env: "production"},
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthcheck", nil)
	app.healthcheckHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 (no auth should be required)", rec.Code)
	}
}
