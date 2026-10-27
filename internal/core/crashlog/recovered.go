// Design: docs/architecture/diagnostics/crash-capture.md -- records of panics a task boundary recovered
// Related: persist.go -- crash file names, the write and the rotation

package crashlog

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// recoveredFileSuffix marks the record of a panic a task boundary recovered while
// the process kept running. Like kernelFileSuffix it is a suffix, so the name
// still sorts by its timestamp and the listing derives the kind from the name.
const recoveredFileSuffix = "-recovered" + crashFileSuffix

// recoveredTimeLayout carries nanoseconds. A process that crashes writes one
// report, but a process that keeps running can recover two panics in the same
// second, and a second-resolution name would let the later report overwrite the
// earlier one.
const recoveredTimeLayout = "20060102-150405.000000000"

// ErrNoCrashDir reports that no crash directory was resolved, so a recovered
// panic has no file to be written to. Init resolves the directory; a process
// that never called Init, or found no writable candidate, has none.
var ErrNoCrashDir = errors.New("no crash directory: set ze.crash.dir to a writable directory")

// RecordRecoveredPanic writes the crash report of a panic a task boundary
// recovered, and sends one line to syslog when a destination is configured. The
// process keeps running, so nothing is printed to stderr: the boundary's own log
// line carries the stack to the log.
//
// origin is one line naming what the panic ended (component, task, unit). The
// report carries origin, the panic value, the stack and the recent log, and is
// listed by `show crashes` with KindRecovered.
//
// It returns the file name `show crashes name <file>` reads, or an error when no
// record could be written. The caller MUST report that error, because a missing
// report is otherwise invisible to the operator who goes looking for it.
//
// Safe for concurrent use: each call writes its own file.
func RecordRecoveredPanic(origin string, value any, stack []byte) (string, error) {
	if syslogAddr != "" {
		var msg []byte
		msg = append(msg, "PANIC (recovered): "...)
		msg = append(msg, origin...)
		msg = append(msg, ": "...)
		msg = appendValue(msg, value)
		writeSyslogCrash(syslogAddr, string(msg))
	}

	if crashDir == "" {
		return "", ErrNoCrashDir
	}

	report := buildCrashReport(value, stack) +
		"\n=== Recovered Panic ===\nOrigin: " + origin +
		"\nThe process kept running. Only the unit named above was ended.\n"

	name := crashFilePrefix + time.Now().UTC().Format(recoveredTimeLayout) + recoveredFileSuffix
	if err := writeCrashFileNamed(crashDir, name, report); err != nil {
		return "", err
	}

	rotateRecoveredFiles(crashDir, crashKeep)
	return name, nil
}

// isRecoveredFile reports whether name is the record of a recovered panic.
func isRecoveredFile(name string) bool {
	return strings.HasSuffix(name, recoveredFileSuffix)
}

// rotateRecoveredFiles keeps the newest keep recovered records and removes the
// rest. Recovered records rotate apart from process crashes, so a peer that
// triggers the same contained panic on every reconnect cannot evict the report
// of the crash that last ended the process.
func rotateRecoveredFiles(dir string, keep int) {
	var recovered []string
	for _, name := range listCrashFileNames(dir) {
		if isRecoveredFile(name) {
			recovered = append(recovered, name)
		}
	}
	if len(recovered) <= keep {
		return
	}

	for _, name := range recovered[:len(recovered)-keep] {
		os.Remove(filepath.Join(dir, name)) //nolint:errcheck // best-effort rotation
	}
}
