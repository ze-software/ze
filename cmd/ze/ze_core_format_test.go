// Design: docs/architecture/api/commands.md -- ze format tests

//go:build ze_core

package main

import (
	"testing"
)

func TestRunFormatHelp(t *testing.T) {
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
			if got := runFormat(tt.args); got != tt.want {
				t.Errorf("runFormat(%v) = %d, want %d", tt.args, got, tt.want)
			}
		})
	}
}

func TestRunFormatInvalidOperator(t *testing.T) {
	if got := runFormat([]string{"nosuchop"}); got != 1 {
		t.Errorf("runFormat(nosuchop) = %d, want 1", got)
	}
}
