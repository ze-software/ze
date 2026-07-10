// VALIDATES: parseLevelArg maps kmsg level names and 0..7 numerics to their level
// index and falls back to 7 (debug) for anything else, and isEAGAIN unwraps
// wrapped syscall.EAGAIN errors.
// PREVENTS: a mis-parsed kernel-log level filter (silently widening/narrowing the
// output) or a missed EAGAIN causing a non-blocking read to be treated as fatal.

//go:build linux

package cmd

import (
	"errors"
	"fmt"
	"syscall"
	"testing"
)

func TestParseLevelArg(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want int
	}{
		{"emerg", 0},
		{"err", 3},
		{"warning", 4},
		{"debug", 7},
		{"0", 0},
		{"5", 5},
		{"7", 7},
		{"8", 7},     // numeric out of range → fallback
		{"-1", 7},    // negative → fallback
		{"bogus", 7}, // unknown name → fallback
		{"", 7},
	} {
		if got := parseLevelArg(tc.in); got != tc.want {
			t.Errorf("parseLevelArg(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestIsEAGAIN(t *testing.T) {
	if !isEAGAIN(syscall.EAGAIN) {
		t.Error("isEAGAIN(EAGAIN) = false, want true")
	}
	if !isEAGAIN(fmt.Errorf("read failed: %w", syscall.EAGAIN)) {
		t.Error("isEAGAIN(wrapped EAGAIN) = false, want true")
	}
	if isEAGAIN(nil) {
		t.Error("isEAGAIN(nil) = true, want false")
	}
	if isEAGAIN(errors.New("boom")) {
		t.Error("isEAGAIN(unrelated) = true, want false")
	}
}
