package main

import "testing"

// TestRejectSecretFlags_Cases enumerates every shape of secret-bearing flag
// the audit told us to refuse at startup. The function MUST return a
// non-empty raw value for each, because the caller treats any non-empty
// return as a fatal startup error. False positives (the second half of the
// table, where the helper must NOT match) are equally important: a flag
// like -api-url-alphavantage shares a prefix with -api-key-alphavantage
// and would block legitimate launches if matched here.
func TestRejectSecretFlags_Cases(t *testing.T) {
	cases := []struct {
		name      string
		args      []string
		wantRaw   string
		wantMatch string
	}{
		// Single-dash forms.
		{"single-dash, value-after-equals, encryption-key", []string{"-encryption-key=deadbeef"}, "-encryption-key=deadbeef", "encryption-key"},
		{"single-dash, value-as-next-arg, encryption-key", []string{"-encryption-key", "deadbeef"}, "-encryption-key", "encryption-key"},
		{"single-dash, redis-password", []string{"-redis-password=hunter2"}, "-redis-password=hunter2", "redis-password"},
		{"single-dash, smtp-password", []string{"-smtp-password=swordfish"}, "-smtp-password=swordfish", "smtp-password"},
		{"single-dash, db-dsn embeds creds", []string{"-db-dsn=postgres://u:p@h/d"}, "-db-dsn=postgres://u:p@h/d", "db-dsn"},
		{"single-dash, api-key-alphavantage", []string{"-api-key-alphavantage=abc"}, "-api-key-alphavantage=abc", "api-key-alphavantage"},
		{"single-dash, api-key-groq", []string{"-api-key-groq=xyz"}, "-api-key-groq=xyz", "api-key-groq"},

		// Double-dash forms (Go flag accepts both).
		{"double-dash, encryption-key", []string{"--encryption-key=deadbeef"}, "--encryption-key=deadbeef", "encryption-key"},
		{"double-dash, api-key-fred", []string{"--api-key-fred=val"}, "--api-key-fred=val", "api-key-fred"},

		// Mixed positional - the secret arg can be anywhere.
		{"secret in second position", []string{"-port=4000", "-encryption-key=x"}, "-encryption-key=x", "encryption-key"},

		// False positives the helper MUST NOT trigger on. -api-url-* shares
		// a prefix root with -api-key-* but is harmless metadata.
		{"clean: api-url is not api-key", []string{"-api-url-alphavantage=https://example.com"}, "", ""},
		{"clean: redis-addr is not redis-password", []string{"-redis-addr=localhost:6379"}, "", ""},
		{"clean: smtp-host is not smtp-password", []string{"-smtp-host=mail.example"}, "", ""},
		{"clean: empty args", []string{}, "", ""},
		{"clean: only safe flags", []string{"-port=4000", "-env=production"}, "", ""},
		{"clean: -version flag", []string{"-version"}, "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotRaw, gotMatch := rejectSecretFlags(tc.args)
			if gotRaw != tc.wantRaw {
				t.Errorf("raw = %q, want %q", gotRaw, tc.wantRaw)
			}
			if gotMatch != tc.wantMatch {
				t.Errorf("match = %q, want %q", gotMatch, tc.wantMatch)
			}
		})
	}
}

// TestRejectSecretFlags_StopsAtFirstHit documents that the helper returns
// the FIRST matching arg, not the last. Order matters because the caller's
// error message echoes the matched arg back to the operator; if a launch
// includes two secret flags, naming the first one points the operator at
// the start of the offending command line.
func TestRejectSecretFlags_StopsAtFirstHit(t *testing.T) {
	args := []string{"-port=4000", "-encryption-key=first", "-redis-password=second"}
	raw, match := rejectSecretFlags(args)
	if raw != "-encryption-key=first" {
		t.Errorf("raw = %q, want -encryption-key=first (first hit)", raw)
	}
	if match != "encryption-key" {
		t.Errorf("match = %q, want encryption-key", match)
	}
}
