// Design: docs/architecture/api/commands.md -- ze pipe tests

//go:build ze_core

package main

import (
	"strings"
	"testing"
)

func TestRunPipe(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want int
	}{
		{"no args", nil, 1},
		{"help", []string{"help"}, 0},
		{"dash-h", []string{"-h"}, 0},
		{"dash-dash-help", []string{"--help"}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := runPipe(tt.args); got != tt.want {
				t.Errorf("runPipe(%v) = %d, want %d", tt.args, got, tt.want)
			}
		})
	}
}

func TestRunPipeUnknownOperator(t *testing.T) {
	if got := runPipe([]string{"nosuchop"}); got != 1 {
		t.Errorf("runPipe(nosuchop) = %d, want 1", got)
	}
}

func TestRunPipeIntegerPrecision(t *testing.T) {
	const row = `{"bytes":18446744073709551615,"packets":9007199254740993}`
	const input = `{"rows":[` + row + `]}`
	for _, chain := range []string{"json compact", "ndjson", "resolve | origin | json compact", "display bytes packets | json compact"} {
		t.Run(chain, func(t *testing.T) {
			stdout, stderr, code := runPipeCaptured(t, input, strings.Fields(chain))
			want := "[" + row + "]\n"
			if chain == "ndjson" {
				want = row + "\n"
			}
			if code != 0 || stderr != "" || stdout != want {
				t.Fatalf("runPipe: code %d, stderr %q, stdout %q, want %q", code, stderr, stdout, want)
			}
		})
	}
}
