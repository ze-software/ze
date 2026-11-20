package api

import (
	"strings"
	"testing"
)

// TestBuildInvocationWithoutDeclaration asks what a REST or gRPC command no
// declaration matches becomes once its structured parameters are written:
// each parameter as a "key value" pair after the command, an empty value
// quoted, and whitespace in a key or a value refused (ValidateParams).
//
// Method: buildInvocation with no metadata and no pipe, over the table the
// removed BuildCommand was held to.
func TestBuildInvocationWithoutDeclaration(t *testing.T) {
	tests := []struct {
		name    string
		command string
		params  map[string]string
		want    string
		wantErr bool
	}{
		{name: "no params", command: "show version", params: nil, want: "show version"},
		{name: "empty params map", command: "show version", params: map[string]string{}, want: "show version"},
		{name: "single param", command: "peer", params: map[string]string{"name": "peer1"}, want: "peer name peer1"},
		{name: "explicit empty value quoted", command: "show", params: map[string]string{"prefix": ""}, want: `show prefix ""`},
		{name: "whitespace in key rejected", command: "show", params: map[string]string{"bad key": "val"}, wantErr: true},
		{name: "whitespace in value rejected", command: "show", params: map[string]string{"key": "bad value"}, wantErr: true},
		{name: "tab in key rejected", command: "show", params: map[string]string{"bad\tkey": "val"}, wantErr: true},
		{name: "newline in value rejected", command: "show", params: map[string]string{"key": "bad\nval"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := buildInvocation(tt.command, tt.params, strings.Fields(tt.command), -1, nil)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("buildInvocation() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("buildInvocation() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("buildInvocation() = %q, want %q", got, tt.want)
			}
		})
	}
}
