package main

import (
	"reflect"
	"sort"
	"testing"
)

// TestCorsOptions_OnlyMethodsVary locks in the contract of the
// corsOptions helper introduced when we deduplicated the SSE and API
// CORS configs: the *only* legitimate point of variance between the two
// surfaces is the AllowedMethods slice. Headers, origins source,
// exposed headers, credentials, and max-age must be byte-identical
// across surfaces - any drift here is a policy regression.
//
// If a future PR legitimately needs surface-specific headers or
// max-age, the right path is to extend the helper signature with the
// new parameter, not to reintroduce two struct literals. This test
// would be the place to encode "headers may now differ between
// surfaces" if/when that becomes a real requirement.
func TestCorsOptions_OnlyMethodsVary(t *testing.T) {
	app := &application{
		config: config{
			cors: struct {
				trustedOrigins []string
			}{trustedOrigins: []string{"https://app.example.com"}},
		},
	}

	api := app.corsOptions([]string{"GET", "POST", "PUT", "DELETE", "PATCH"})
	sse := app.corsOptions([]string{"GET"})

	// AllowedMethods is the only field that may differ.
	if reflect.DeepEqual(api.AllowedMethods, sse.AllowedMethods) {
		t.Errorf("API and SSE somehow have identical AllowedMethods; one of them is misconfigured: api=%v sse=%v",
			api.AllowedMethods, sse.AllowedMethods)
	}

	// Every other field must agree byte-for-byte. We compare via copies
	// with AllowedMethods zeroed so DeepEqual catches any other drift
	// without us enumerating each field by name.
	apiCopy := api
	sseCopy := sse
	apiCopy.AllowedMethods = nil
	sseCopy.AllowedMethods = nil
	if !reflect.DeepEqual(apiCopy, sseCopy) {
		t.Errorf("CORS config drifted between surfaces (other than AllowedMethods):\n  api=%+v\n  sse=%+v",
			apiCopy, sseCopy)
	}

	// Spot-check the method sets are what we expect; if a future PR
	// adds a method to the API surface (e.g. HEAD) it should show up in
	// this test and force the author to confirm the change is wanted.
	wantAPI := []string{"DELETE", "GET", "PATCH", "POST", "PUT"}
	gotAPI := append([]string(nil), api.AllowedMethods...)
	sort.Strings(gotAPI)
	if !reflect.DeepEqual(gotAPI, wantAPI) {
		t.Errorf("API AllowedMethods drift: got %v, want %v", gotAPI, wantAPI)
	}
	if !reflect.DeepEqual(sse.AllowedMethods, []string{"GET"}) {
		t.Errorf("SSE AllowedMethods must remain GET-only, got %v", sse.AllowedMethods)
	}

	// AllowCredentials must be false. Setting it to true would require
	// dropping the wildcard origin (browsers reject "*" with credentials)
	// and would cause cross-origin cookie transmission - we authenticate
	// via Bearer tokens, not cookies, so this is a security-relevant
	// invariant.
	if api.AllowCredentials || sse.AllowCredentials {
		t.Error("AllowCredentials must remain false on both surfaces (Bearer-token scheme, no cross-origin cookies)")
	}

	// AllowedOrigins must be sourced from config (this is what makes
	// trusted-origin changes a config push, not a code release).
	if !reflect.DeepEqual(api.AllowedOrigins, []string{"https://app.example.com"}) {
		t.Errorf("AllowedOrigins must mirror app.config.cors.trustedOrigins, got %v", api.AllowedOrigins)
	}
}
