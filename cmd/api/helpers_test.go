package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Blue-Davinci/OptiVest/internal/data"
)

func Test_application_isProfileComplete(t *testing.T) {
	type args struct {
		user *data.User
	}
	tests := []struct {
		name string
		app  *application
		args args
		want bool
	}{
		{
			name: "Complete Profile",
			app:  &application{},
			args: args{
				user: &data.User{
					FirstName:    "John",
					LastName:     "Doe",
					Email:        "john1.doe@example.com",
					PhoneNumber:  "1234567890",
					DOB:          time.Date(1990, time.January, 1, 0, 0, 0, 0, time.UTC),
					Address:      "123 Main St",
					CountryCode:  "US",
					CurrencyCode: "USD",
				},
			},
			want: true,
		},
		{
			name: "Missing FirstName",
			app:  &application{},
			args: args{
				user: &data.User{
					LastName:     "Doe",
					Email:        "john.doe@example.com",
					PhoneNumber:  "1234567890",
					DOB:          time.Date(1990, time.January, 1, 0, 0, 0, 0, time.UTC),
					Address:      "123 Main St",
					CountryCode:  "US",
					CurrencyCode: "USD",
				},
			},
			want: false,
		},
		{
			name: "Missing LastName",
			app:  &application{},
			args: args{
				user: &data.User{
					FirstName:    "John",
					Email:        "john.doe@example.com",
					PhoneNumber:  "1234567890",
					DOB:          time.Date(1990, time.January, 1, 0, 0, 0, 0, time.UTC),
					Address:      "123 Main St",
					CountryCode:  "US",
					CurrencyCode: "USD",
				},
			},
			want: false,
		},
		{
			name: "Missing Email",
			app:  &application{},
			args: args{
				user: &data.User{
					FirstName:    "John",
					LastName:     "Doe",
					PhoneNumber:  "1234567890",
					DOB:          time.Date(1990, time.January, 1, 0, 0, 0, 0, time.UTC),
					Address:      "123 Main St",
					CountryCode:  "US",
					CurrencyCode: "USD",
				},
			},
			want: false,
		},
		{
			name: "Missing PhoneNumber",
			app:  &application{},
			args: args{
				user: &data.User{
					FirstName:    "John",
					LastName:     "Doe",
					Email:        "john.doe@example.com",
					DOB:          time.Date(1990, time.January, 1, 0, 0, 0, 0, time.UTC),
					Address:      "123 Main St",
					CountryCode:  "US",
					CurrencyCode: "USD",
				},
			},
			want: false,
		},
		{
			name: "Missing DOB",
			app:  &application{},
			args: args{
				user: &data.User{
					FirstName:    "John",
					LastName:     "Doe",
					Email:        "john.doe@example.com",
					PhoneNumber:  "1234567890",
					Address:      "123 Main St",
					CountryCode:  "US",
					CurrencyCode: "USD",
				},
			},
			want: false,
		},
		{
			name: "Missing Address",
			app:  &application{},
			args: args{
				user: &data.User{
					FirstName:    "John",
					LastName:     "Doe",
					Email:        "john.doe@example.com",
					PhoneNumber:  "1234567890",
					DOB:          time.Date(1990, time.January, 1, 0, 0, 0, 0, time.UTC),
					CountryCode:  "US",
					CurrencyCode: "USD",
				},
			},
			want: false,
		},
		{
			name: "Missing CountryCode",
			app:  &application{},
			args: args{
				user: &data.User{
					FirstName:    "John",
					LastName:     "Doe",
					Email:        "john.doe@example.com",
					PhoneNumber:  "1234567890",
					DOB:          time.Date(1990, time.January, 1, 0, 0, 0, 0, time.UTC),
					Address:      "123 Main St",
					CurrencyCode: "USD",
				},
			},
			want: false,
		},
		{
			name: "Missing CurrencyCode",
			app:  &application{},
			args: args{
				user: &data.User{
					FirstName:   "John",
					LastName:    "Doe",
					Email:       "john.doe@example.com",
					PhoneNumber: "1234567890",
					DOB:         time.Date(1990, time.January, 1, 0, 0, 0, 0, time.UTC),
					Address:     "123 Main St",
					CountryCode: "US",
				},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.app.isProfileComplete(tt.args.user); got != tt.want {
				t.Errorf("application.isProfileComplete() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestWriteJSON_WireFormat asserts that writeJSON emits a compact-encoded
// JSON body, no trailing newline, and the explicit charset=utf-8
// Content-Type. This is a regression test for the MarshalIndent removal:
// it would have caught any subsequent edit that re-introduces tab
// indentation or a trailing '\n' (both wasted bytes per response).
func TestWriteJSON_WireFormat(t *testing.T) {
	app := &application{}
	rr := httptest.NewRecorder()

	env := envelope{"hello": "world", "n": 42}
	if err := app.writeJSON(rr, http.StatusTeapot, env, nil); err != nil {
		t.Fatalf("writeJSON returned unexpected error: %v", err)
	}

	if got, want := rr.Code, http.StatusTeapot; got != want {
		t.Errorf("status: got %d, want %d", got, want)
	}

	if got, want := rr.Header().Get("Content-Type"), "application/json; charset=utf-8"; got != want {
		t.Errorf("Content-Type: got %q, want %q", got, want)
	}

	body := rr.Body.String()

	// No trailing newline. A trailing '\n' is one wasted byte per
	// response and is not part of the JSON wire format.
	if strings.HasSuffix(body, "\n") {
		t.Errorf("body unexpectedly ends with a newline: %q", body)
	}

	// No tab indentation. MarshalIndent injected '\t' between keys; any
	// recurrence of that pattern means MarshalIndent has crept back in.
	if strings.Contains(body, "\t") {
		t.Errorf("body unexpectedly contains tab characters: %q", body)
	}

	// Round-trip parse: whatever bytes we emitted must still decode to
	// the same envelope. This catches a class of defects where someone
	// "compacts" the output by removing whitespace incorrectly and ends
	// up emitting invalid JSON.
	var got map[string]any
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatalf("emitted body did not parse as JSON: %v\nbody=%q", err, body)
	}
	if got["hello"] != "world" {
		t.Errorf(`expected hello=="world", got %v`, got["hello"])
	}
}

// TestWriteJSON_HeadersMerged covers the caller-supplied-header path:
// writeJSON must NOT overwrite headers the caller has already set on
// itself (e.g. Cache-Control, X-Request-ID), and must NOT silently drop
// them either. Using a Set on Content-Type after merging the caller map
// is the documented behavior - callers can't override Content-Type via
// this path, which is correct because the body is JSON.
func TestWriteJSON_HeadersMerged(t *testing.T) {
	app := &application{}
	rr := httptest.NewRecorder()

	custom := http.Header{}
	custom.Set("Cache-Control", "no-store")
	custom.Set("X-Custom", "value")

	if err := app.writeJSON(rr, http.StatusOK, envelope{"ok": true}, custom); err != nil {
		t.Fatalf("writeJSON returned unexpected error: %v", err)
	}

	if got := rr.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control not propagated: got %q", got)
	}
	if got := rr.Header().Get("X-Custom"); got != "value" {
		t.Errorf("X-Custom not propagated: got %q", got)
	}
	// Content-Type wins regardless of what the caller passed.
	if got := rr.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Errorf("Content-Type should be set by writeJSON, got %q", got)
	}
}
