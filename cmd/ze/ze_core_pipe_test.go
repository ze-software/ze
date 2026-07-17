// Design: docs/architecture/api/commands.md -- ze pipe tests

//go:build ze_core

package main

import (
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
