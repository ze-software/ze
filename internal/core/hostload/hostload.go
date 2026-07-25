// Design: docs/architecture/testing/runner-architecture.md -- host load detection for contended-run classification

// Package hostload samples system load to classify whether a test run was
// CPU-contended. It is the single source of truth for the "contended" verdict,
// shared by the functional-test runner (internal/test/runner) and the verify
// status tool (scripts/status) so the two surfaces cannot drift on what
// "contended" means.
package hostload

import (
	"context"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

const procTimeout = 2 * time.Second

// Load captures a snapshot of system load at a point in time.
// Used to classify whether a test run was contended.
type Load struct {
	LoadAvg1    float64 `json:"load-avg-1"`
	CPUs        int     `json:"cpus"`
	ZeProcs     int     `json:"ze-procs"`
	GoTestProcs int     `json:"go-test-procs"`
}

// Contended returns true when system load suggests CPU starvation.
// The threshold is load-avg-1 > CPUs (fully loaded) AND at least one
// concurrent ze or go-test process besides the caller.
func (l Load) Contended() bool {
	return l.LoadAvg1 > float64(l.CPUs) && (l.ZeProcs > 1 || l.GoTestProcs > 0)
}

// String returns a compact summary for log output.
func (l Load) String() string {
	var tb textbuf.Buffer
	return tb.Str("load=").Float(l.LoadAvg1, 1).
		Str(" cpus=").Int(int64(l.CPUs)).
		Str(" ze=").Int(int64(l.ZeProcs)).
		Str(" gotest=").Int(int64(l.GoTestProcs)).String()
}

// Snapshot samples the current host load.
// Returns a zero Load (Contended() == false) if sampling fails.
func Snapshot() Load {
	l := Load{
		CPUs: runtime.NumCPU(),
	}
	l.LoadAvg1 = readLoadAvg1()
	l.ZeProcs = processCount("ze-test")
	l.GoTestProcs = processCount("\\.test")
	return l
}

// processCount counts processes whose comm field matches pattern.
// Uses "ps -eo comm" piped to "grep -c" which works on both macOS and Linux
// (pgrep -c and pgrep -f are Linux-only extensions).
func processCount(pattern string) int {
	ctx, cancel := context.WithTimeout(context.Background(), procTimeout)
	defer cancel()
	var tb textbuf.Buffer
	shellCmd := tb.Str("ps -eo comm | grep -c '").Str(pattern).Byte('\'').String()
	cmd := exec.CommandContext(ctx, "sh", "-c", shellCmd) //nolint:gosec // pattern is a compile-time constant
	out, err := cmd.Output()
	if err != nil {
		return 0
	}
	return parseDigits(string(out))
}

// parseDigits extracts the leading integer from s.
// Stops at the first non-digit to avoid interpreting error messages as counts.
func parseDigits(s string) int {
	s = strings.TrimSpace(s)
	n := 0
	for _, b := range s {
		if b < '0' || b > '9' {
			break
		}
		n = n*10 + int(b-'0')
	}
	return n
}
