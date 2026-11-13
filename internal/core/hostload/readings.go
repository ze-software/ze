// Design: docs/architecture/testing/runner-architecture.md -- host load detection for contended-run classification
// Overview: hostload.go -- Snapshot, whose failed sample is a zero by contract
// Detail: parse.go -- the parsers of what each platform prints
//
// The readings that report failure. Snapshot answers zero for a sample it
// could not take, which is right for its one question (was this run
// contended) and wrong for a reader who shows the number: a zero load on a
// machine whose load could not be read looks like an idle machine. Each
// function here answers its value, or the error that says why there is none.

package hostload

import (
	"errors"
	"strings"
)

// goTestSuffix is what the name of a compiled Go test binary holds: `go test`
// builds `<package>.test` and runs it.
const goTestSuffix = ".test"

// LoadAverages answers the one, five and fifteen minute load averages, or the
// error that says why they could not be read. It never answers a zero for a
// failed reading.
//
// Safe for concurrent use.
func LoadAverages() (Averages, error) {
	return readLoadAverages()
}

// MemoryTotal answers the machine's physical memory in bytes, or the error
// that says why it could not be read.
//
// Safe for concurrent use.
func MemoryTotal() (uint64, error) {
	return readMemoryTotal()
}

// MemoryAvailable answers, in bytes, the memory the system can give a new
// process without swapping, or the error that says why it could not be read.
//
// On Linux it is the kernel's own estimate, MemAvailable of /proc/meminfo. On
// macOS it is the free, speculative and inactive pages `vm_stat` counts,
// which is an upper estimate (parseVMStatAvailable states it exactly).
//
// Safe for concurrent use.
func MemoryAvailable() (uint64, error) {
	return readMemoryAvailable()
}

// HarnessProcesses counts the running `le test <name>` commands on the whole
// machine, the caller among them when it is one, or answers the error that
// says why the process list could not be read.
//
// Safe for concurrent use.
func HarnessProcesses() (int, error) {
	lines, err := processArguments()
	if err != nil {
		return 0, err
	}
	count := 0
	for _, line := range lines {
		if isHarnessCommand(line) {
			count++
		}
	}
	return count, nil
}

// GoTestProcesses counts the running compiled Go test binaries on the whole
// machine, or answers the error that says why the process list could not be
// read. It counts what Snapshot counts: a process whose name holds ".test".
//
// Safe for concurrent use.
func GoTestProcesses() (int, error) {
	names, err := processColumn("comm=")
	if err != nil {
		return 0, err
	}
	return countGoTests(names), nil
}

// countGoTests counts the process names that are compiled Go test binaries.
func countGoTests(names []string) int {
	count := 0
	for _, name := range names {
		if strings.Contains(name, goTestSuffix) {
			count++
		}
	}
	return count
}

// processLines splits what `ps` printed into one entry per process. A list
// with no process in it is a failed reading: `ps` itself is always running
// while it prints.
func processLines(printed string) ([]string, error) {
	trimmed := strings.TrimSpace(printed)
	if trimmed == "" {
		return nil, errors.New("ps printed no process")
	}
	return strings.Split(trimmed, "\n"), nil
}
