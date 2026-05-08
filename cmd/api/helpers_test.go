package main

import (
	"encoding/json"
	"errors"
	"fmt"
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

// TestValidateJSONContentType_Accepted covers every form of
// `application/json` that real-world HTTP clients send so we can be
// confident the new readJSON gate doesn't 415 a client that's actually
// well-behaved. Each row should pass cleanly (return nil); any failure
// is a regression that would break a real frontend at runtime.
func TestValidateJSONContentType_Accepted(t *testing.T) {
	cases := []struct {
		name string
		ct   string
	}{
		{"bare", "application/json"},
		{"with-utf8-charset", "application/json; charset=utf-8"},
		{"with-uppercase-charset", "application/json; charset=UTF-8"},
		{"with-extra-params", "application/json; charset=utf-8; boundary=ignored"},
		{"with-leading-whitespace-around-params", "application/json ; charset=utf-8"},
		{"uppercase-media-type", "APPLICATION/JSON"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader("{}"))
			r.Header.Set("Content-Type", c.ct)
			if err := validateJSONContentType(r); err != nil {
				t.Errorf("expected accept for %q, got error: %v", c.ct, err)
			}
		})
	}
}

// TestValidateJSONContentType_Rejected covers the failure modes. Every
// row must return an error that errors.Is unwraps to
// ErrUnsupportedMediaType, since badRequestResponse routes on that
// sentinel to emit 415 rather than 400. A failure to wrap correctly
// would silently fall through to 400, defeating the whole point.
func TestValidateJSONContentType_Rejected(t *testing.T) {
	cases := []struct {
		name string
		ct   string
	}{
		{"missing", ""},
		{"plain-text", "text/plain"},
		{"xml", "application/xml"},
		{"form-encoded", "application/x-www-form-urlencoded"},
		{"multipart", "multipart/form-data; boundary=foo"},
		{"malformed", "definitely not a media type"},
		{"json-suffix-only", "application/jsonish"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader("{}"))
			if c.ct != "" {
				r.Header.Set("Content-Type", c.ct)
			}
			err := validateJSONContentType(r)
			if err == nil {
				t.Fatalf("expected reject for %q, got nil", c.ct)
			}
			if !errors.Is(err, ErrUnsupportedMediaType) {
				t.Errorf("error must wrap ErrUnsupportedMediaType so badRequestResponse routes to 415; got: %v", err)
			}
		})
	}
}

// TestBadRequestResponse_RoutesUnsupportedMediaType locks in the
// auto-routing behavior: any error chain that contains
// ErrUnsupportedMediaType must come out as 415, regardless of how it was
// wrapped. This is what lets the existing ~50 readJSON call sites stay
// untouched - they already do
//
//	if err != nil { app.badRequestResponse(w, r, err); return }
//
// and that path now correctly emits 415 for content-type errors and
// 400 for everything else.
func TestBadRequestResponse_RoutesUnsupportedMediaType(t *testing.T) {
	app := &application{}

	t.Run("wrapped sentinel -> 415", func(t *testing.T) {
		rr := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/x", nil)
		err := fmt.Errorf("%w: got %q", ErrUnsupportedMediaType, "text/plain")
		app.badRequestResponse(rr, r, err)
		if got, want := rr.Code, http.StatusUnsupportedMediaType; got != want {
			t.Errorf("status: got %d, want %d", got, want)
		}
		// Body must NOT echo the rejected Content-Type back to the client.
		// The diagnostic context is server-side only; the wire envelope
		// is intentionally constant.
		if strings.Contains(rr.Body.String(), "text/plain") {
			t.Errorf("response unexpectedly echoes the rejected media type: %q", rr.Body.String())
		}
	})

	t.Run("plain error -> 400", func(t *testing.T) {
		rr := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/x", nil)
		app.badRequestResponse(rr, r, errors.New("body contains badly-formed JSON"))
		if got, want := rr.Code, http.StatusBadRequest; got != want {
			t.Errorf("status: got %d, want %d", got, want)
		}
	})
}

// TestReadJSON_RejectsWrongContentType is the integration check that
// the readJSON guard composes with the auto-routing. We feed in a body
// that WOULD parse cleanly (valid JSON) but with the wrong Content-Type;
// the rejection must happen at the validation gate, before any body
// bytes are consumed.
func TestReadJSON_RejectsWrongContentType(t *testing.T) {
	app := &application{}
	rr := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(`{"ok":true}`))
	r.Header.Set("Content-Type", "text/plain")
	var dst struct {
		OK bool `json:"ok"`
	}
	err := app.readJSON(rr, r, &dst)
	if err == nil {
		t.Fatal("expected readJSON to reject text/plain body, got nil")
	}
	if !errors.Is(err, ErrUnsupportedMediaType) {
		t.Errorf("returned error must wrap ErrUnsupportedMediaType; got: %v", err)
	}
	// Body must remain unread - confirm dst is still the zero value.
	if dst.OK {
		t.Error("readJSON consumed the body despite content-type rejection (dst was decoded)")
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
