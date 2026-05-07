package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.uber.org/zap"
)

// TestAccountRecoveryEligibleResponse_WireFormat asserts the canonical
// shape every unauthenticated account-recovery / activation / password-
// reset handler must produce. The wire MUST be byte-identical regardless
// of which branch the caller took (email-not-found, already-activated,
// MFA-not-enabled, or the genuine happy path); if this test ever needs
// to change, every leak-prone caller almost certainly does too.
//
// The previous shapes were a 422 with v.Errors envelope (email-not-found
// branches), a 423 Locked (inactive-account branch), and a bespoke
// "if we found a matching email..." message on the password-reset happy
// path - three observably-different responses that doubled as account-
// state oracles for an anonymous probe.
func TestAccountRecoveryEligibleResponse_WireFormat(t *testing.T) {
	app := &application{logger: zap.NewNop()}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/api/password-reset", nil)
	app.accountRecoveryEligibleResponse(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Errorf("Content-Type = %q, want application/json prefix", got)
	}

	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("response body is not valid JSON: %v\nbody: %s", err, rec.Body.String())
	}
	msg, ok := body["message"].(string)
	if !ok {
		t.Fatalf("body missing string field 'message'; got %#v", body)
	}
	wantPrefix := "if your account is eligible"
	if !strings.HasPrefix(msg, wantPrefix) {
		t.Errorf("message = %q, want prefix %q", msg, wantPrefix)
	}
	// Negative checks: previous handler-specific phrasings must NOT survive.
	for _, leak := range []string{
		"no matching email address found",
		"user has already been activated",
		"user account has not been activated",
		"user account has not enabled MFA",
		"if we found a matching email address",
	} {
		if strings.Contains(msg, leak) {
			t.Errorf("message %q contains leaky phrase %q", msg, leak)
		}
	}
}
