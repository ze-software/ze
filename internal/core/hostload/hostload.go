// Design: docs/architecture/testing/runner-architecture.md -- host load detection for contended-run classification

// Package hostload samples system load to classify whether a test run was
// CPU-contended. It is the single source of truth for the "contended" verdict,
// shared by the functional-test runner (internal/test/runner) and the native
// verification-status action so the two surfaces cannot drift on what
// "contended" means.
//
// It also holds the machine readings a report shows a person (readings.go):
// the three load averages, total and available memory, and the two process
// counts. Those report a failed reading as an error, where Snapshot answers
// zero.
package hostload

import (
	"context"
	"errors"
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
	l.LoadAvg1 = loadAvg1()
	l.ZeProcs = harnessProcessCount()
	l.GoTestProcs = goTestProcessCount()
	return l
}

// loadAvg1 answers the one-minute load average for Snapshot. A failed sample
// answers 0, as Snapshot documents; LoadAverages is the reading that reports
// the failure instead.
func loadAvg1() float64 {
	averages, err := readLoadAverages()
	// refuse-by-default: guards Contended -- a zero load is the uncontended verdict Snapshot documents for a failed sample
	if err != nil {
		return 0
	}
	return averages.One
}

// harnessProcessCount counts the running test-harness processes: every
// `le test <name>` command, the runner that called Snapshot among them.
//
// The harness has no binary of its own (`le test <name>` is an ordinary le
// command), so the process name reads `le` for every le command and only the
// argument list tells the harness apart. `ps -eo args=` prints it on macOS and
// Linux alike. A failed sample answers 0, as Snapshot documents;
// HarnessProcesses is the reading that reports the failure instead.
func harnessProcessCount() int {
	count, err := HarnessProcesses()
	// refuse-by-default: guards Contended -- no counted harness is the uncontended verdict Snapshot documents for a failed sample
	if err != nil {
		return 0
	}
	return count
}

// processArguments answers the argument list of every process, one per entry.
func processArguments() ([]string, error) {
	return processColumn("args=")
}

// processColumn answers one `ps` column for every process, one per entry. The
// trailing "=" of the column drops the header line.
func processColumn(column string) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), procTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "ps", "-eo", column).Output() //nolint:gosec // the column is a constant of this package
	if err != nil {
		var tb textbuf.Buffer
		return nil, errors.New(tb.Str("ps -eo ").Str(column).Str(": ").Err(err).String())
	}
	return processLines(string(out))
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

// goTestProcessCount counts the running compiled Go test binaries for
// Snapshot. It is GoTestProcesses, the one counter of them, so the report and
// the contended verdict cannot count two ways. A failed sample answers 0, as
// Snapshot documents; GoTestProcesses is the reading that reports the failure
// instead.
func goTestProcessCount() int {
	count, err := GoTestProcesses()
	// refuse-by-default: guards Contended -- no counted Go test is the uncontended verdict Snapshot documents for a failed sample
	if err != nil {
		return 0
	}
	return count
}
