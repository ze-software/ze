// Design: docs/architecture/testing/runner-architecture.md -- host load detection for contended-run classification

package runner

import (
	"context"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/core/textbuf"
)

const procTimeout = 2 * time.Second

const (
	// FailTypeNearTimeout classifies a failure where the test consumed >80%
	// of its timeout budget without the context deadline actually firing.
	// Distinguishes CPU-starvation near-misses from genuine "unknown" failures.
	FailTypeNearTimeout = "near_timeout"

	// nearTimeoutThreshold is the fraction of timeout elapsed above which
	// a non-specific failure is reclassified as near-timeout.
	nearTimeoutThreshold = 0.80
)

// HostLoad captures a snapshot of system load at a point in time.
// Used to classify whether a test run was contended.
type HostLoad struct {
	LoadAvg1    float64 `json:"load-avg-1"`
	CPUs        int     `json:"cpus"`
	ZeProcs     int     `json:"ze-procs"`
	GoTestProcs int     `json:"go-test-procs"`
}

// Contended returns true when system load suggests CPU starvation.
// The threshold is load-avg-1 > CPUs (fully loaded) AND at least one
// concurrent ze or go-test process besides the caller.
func (h HostLoad) Contended() bool {
	return h.LoadAvg1 > float64(h.CPUs) && (h.ZeProcs > 1 || h.GoTestProcs > 0)
}

// String returns a compact summary for log output.
func (h HostLoad) String() string {
	var tb textbuf.Buffer
	return tb.Str("load=").Float(h.LoadAvg1, 1).
		Str(" cpus=").Int(int64(h.CPUs)).
		Str(" ze=").Int(int64(h.ZeProcs)).
		Str(" gotest=").Int(int64(h.GoTestProcs)).String()
}

// IsNearTimeout returns true when a test failure should be reclassified
// from "unknown" to "near_timeout". This happens when the test consumed
// more than 80% of its timeout budget and the failure type is non-specific
// (empty or "unknown"), indicating CPU starvation rather than a real bug.
func IsNearTimeout(elapsedRatio float64, failureType string) bool {
	if elapsedRatio <= nearTimeoutThreshold {
		return false
	}
	return failureType == "" || failureType == stateUnknown
}

// SnapshotHostLoad samples the current host load.
// Returns a zero HostLoad (Contended() == false) if sampling fails.
func SnapshotHostLoad() HostLoad {
	h := HostLoad{
		CPUs: runtime.NumCPU(),
	}
	h.LoadAvg1 = readLoadAvg1()
	h.ZeProcs = grepProcessCount("ze-test")
	h.GoTestProcs = grepProcessCount("\\.test")
	return h
}

// grepProcessCount counts processes whose comm field matches pattern.
// Uses "ps -eo comm" piped to "grep -c" which works on both macOS and Linux
// (pgrep -c and pgrep -f are Linux-only extensions).
func grepProcessCount(pattern string) int {
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
