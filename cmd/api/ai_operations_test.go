package main

import (
	"reflect"
	"testing"
)

func Test_parseLLMResponse(t *testing.T) {
	tests := []struct {
		name       string
		response   string
		wantTop    string
		wantJSON   map[string]interface{}
		wantBottom string
		wantErr    bool
	}{
		{
			name:     "Valid response with top, JSON, and bottom",
			response: "Top section\n```json\n{\"key\": \"value\"}\n```Bottom section",
			wantTop:  "Top section",
			wantJSON: map[string]interface{}{
				"key": "value",
			},
			wantBottom: "Bottom section",
			wantErr:    false,
		},
		{
			name:       "No JSON section found",
			response:   "Top section without JSON",
			wantTop:    "",
			wantJSON:   nil,
			wantBottom: "",
			wantErr:    true,
		},
		{
			name:       "Incomplete JSON section",
			response:   "Top section\n```json\n{\"key\": \"value\"",
			wantTop:    "",
			wantJSON:   nil,
			wantBottom: "",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotTop, gotJSON, gotBottom, err := parseLLMResponse(tt.response)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseLLMResponse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotTop != tt.wantTop {
				t.Errorf("parseLLMResponse() gotTop = %v, wantTop %v", gotTop, tt.wantTop)
			}
			if !reflect.DeepEqual(gotJSON, tt.wantJSON) {
				t.Errorf("parseLLMResponse() gotJSON = %v, wantJSON %v", gotJSON, tt.wantJSON)
			}
			if gotBottom != tt.wantBottom {
				t.Errorf("parseLLMResponse() gotBottom = %v, wantBottom %v", gotBottom, tt.wantBottom)
			}
		})
	}
}
