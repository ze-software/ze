package api

import (
	"testing"
)

func TestBuildCommand(t *testing.T) {
	tests := []struct {
		name    string
		command string
		params  map[string]string
		want    string
		wantErr bool
	}{
		{
			name:    "no params",
			command: "show version",
			params:  nil,
			want:    "show version",
		},
		{
			name:    "empty params map",
			command: "show version",
			params:  map[string]string{},
			want:    "show version",
		},
		{
			name:    "single param",
			command: "peer",
			params:  map[string]string{"name": "peer1"},
			want:    "peer name peer1",
		},
		{
			name:    "empty value skipped",
			command: "show",
			params:  map[string]string{"prefix": ""},
			want:    "show",
		},
		{
			name:    "whitespace in key rejected",
			command: "show",
			params:  map[string]string{"bad key": "val"},
			wantErr: true,
		},
		{
			name:    "whitespace in value rejected",
			command: "show",
			params:  map[string]string{"key": "bad value"},
			wantErr: true,
		},
		{
			name:    "tab in key rejected",
			command: "show",
			params:  map[string]string{"bad\tkey": "val"},
			wantErr: true,
		},
		{
			name:    "newline in value rejected",
			command: "show",
			params:  map[string]string{"key": "bad\nval"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BuildCommand(tt.command, tt.params)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("BuildCommand() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("BuildCommand() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("BuildCommand() = %q, want %q", got, tt.want)
			}
		})
	}
}
