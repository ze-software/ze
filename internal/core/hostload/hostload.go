// Design: docs/architecture/testing/runner-architecture.md -- host load detection for contended-run classification

// Package hostload samples system load to classify whether a test run was
// CPU-contended. It is the single source of truth for the "contended" verdict,
// shared by the functional-test runner (internal/test/runner) and the native
// verification-status action so the two surfaces cannot drift on what
// "contended" means.
package hostload

import (
	"context"
	"os/exec"
	"path/filepath"
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
// concurrent `le test` or go-test process besides the caller.
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
	l.ZeProcs = harnessProcessCount()
	l.GoTestProcs = processCount("\\.test")
	return l
}

// harnessProcessCount counts the running test-harness processes: every
// `le test <name>` command, the runner that called Snapshot among them.
//
// The harness has no binary of its own (`le test <name>` is an ordinary le
// command), so the process name reads `le` for every le command and only the
// argument list tells the harness apart. `ps -eo args=` prints it on macOS and
// Linux alike. A failed sample answers 0, as Snapshot documents.
func harnessProcessCount() int {
	lines, err := processArguments()
	if err != nil {
		return 0
	}
	count := 0
	for _, line := range lines {
		if isHarnessCommand(line) {
			count++
		}
	}
	return count
}

// processArguments answers the argument list of every process, one per entry.
func processArguments() ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), procTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "ps", "-eo", "args=").Output()
	if err != nil {
		return nil, err
	}
	return strings.Split(strings.TrimSpace(string(out)), "\n"), nil
}

// isHarnessCommand answers whether one `ps` argument list is an `le test`
// command: the program's base name is `le` and its first argument is `test`.
func isHarnessCommand(args string) bool {
	fields := strings.Fields(args)
	if len(fields) < 2 {
		return false
	}
	if filepath.Base(fields[0]) != "le" {
		return false
	}
	return fields[1] == "test"
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
