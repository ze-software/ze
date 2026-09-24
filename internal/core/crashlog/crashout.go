// Design: docs/architecture/diagnostics/crash-capture.md -- the runtime's own crash file, and the harvest on the next start
// Related: stderr.go -- why descriptor 2 is left to the runtime
// Related: persist.go -- the crash file names and the rotation the harvest joins

package crashlog

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"time"
)

// pendingDirName is the directory under the crash directory that holds one
// armed crash output file per running process.
const pendingDirName = ".pending"

// runtimeOutputMarker ends the header armCrashOutput writes. A pending file with
// nothing after it belonged to a process that ended without a fatal error.
const runtimeOutputMarker = "\n=== Runtime Output ===\n"

// crashOut is this process's armed crash output file. It stays open for the life
// of the process, because its lock is what tells a harvest the process is alive.
var crashOut *os.File //nolint:unused // held, never read: the reference keeps the armed file open, and no read expresses that

// armCrashOutput hands the Go runtime a file of its own in dir, where it writes a
// copy of every fatal error, unrecovered panic and SIGQUIT dump.
//
// The runtime writes that copy itself, with the world stopped, so no goroutine of
// Ze's has to run for the crash to be recorded. That is the only kind of capture
// that works at that moment: the stderr relay is frozen with every other
// goroutine, and the process exits before it could ever run again.
//
// The file cannot be created when the crash happens, so it is created now, in
// dir/.pending, and locked. The next process to start harvests it: a file whose
// owner is gone and that holds runtime output becomes a crash file, and one that
// holds only the header is removed.
func armCrashOutput(dir string) error {
	pending := filepath.Join(dir, pendingDirName)
	if err := os.MkdirAll(pending, 0o700); err != nil {
		return err
	}
	f, err := os.CreateTemp(pending, "run-*"+crashFileSuffix)
	if err != nil {
		return err
	}
	if err := lockFile(f); err != nil {
		return discardPending(f, err)
	}
	if _, err := f.Write(appendRuntimeHeader(nil)); err != nil {
		return discardPending(f, err)
	}
	if err := debug.SetCrashOutput(f, debug.CrashOptions{}); err != nil {
		return discardPending(f, err)
	}
	crashOut = f
	return nil
}

// discardPending removes a pending file that could not be armed and returns why.
func discardPending(f *os.File, cause error) error {
	f.Close()           //nolint:errcheck // the file is being discarded
	os.Remove(f.Name()) //nolint:errcheck // the file is being discarded
	return cause
}

// appendRuntimeHeader writes what is known at start into the pending file. The
// release version is not among it: Init runs before the version is stamped.
func appendRuntimeHeader(b []byte) []byte {
	b = append(b, "=== Ze Crash Report ===\n"...)
	b = append(b, "Kind: Go runtime fatal output (unrecovered panic, fatal error, or SIGQUIT dump)\n"...)
	b = append(b, "Started: "...)
	b = append(b, time.Now().UTC().Format(time.RFC3339)...)
	b = appendBuildCommit(b)
	b = append(b, "\nGo: "...)
	b = append(b, runtime.Version()...)
	b = append(b, "\nOS/Arch: "...)
	b = append(b, runtime.GOOS...)
	b = append(b, '/')
	b = append(b, runtime.GOARCH...)
	b = append(b, "\nPID: "...)
	b = strconv.AppendInt(b, int64(os.Getpid()), 10)
	b = appendCommand(b)
	b = append(b, '\n')
	b = append(b, runtimeOutputMarker...)
	return b
}

// harvestPendingCrashes turns every pending file whose owner is gone into a
// crash file, or removes it when its owner ended cleanly. A pending file whose
// lock is still held belongs to a running process and is left alone.
func harvestPendingCrashes(dir string, keep int) {
	pending := filepath.Join(dir, pendingDirName)
	entries, err := os.ReadDir(pending)
	if err != nil {
		return
	}
	harvested := false
	for _, e := range entries {
		if !e.Type().IsRegular() || !strings.HasSuffix(e.Name(), crashFileSuffix) {
			continue
		}
		if harvestPending(dir, filepath.Join(pending, e.Name())) {
			harvested = true
		}
	}
	if harvested {
		rotateCrashFiles(dir, keep)
	}
}

// harvestPending handles one pending file, and reports whether it became a crash
// file.
func harvestPending(dir, path string) bool {
	f, err := os.Open(path) //nolint:gosec // path is an entry of the crash directory's pending directory
	if err != nil {
		return false
	}
	defer f.Close() //nolint:errcheck // read-only
	if err := lockFile(f); err != nil {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	data, err := io.ReadAll(f)
	if err != nil {
		return false
	}
	if !holdsRuntimeOutput(data) {
		os.Remove(path) //nolint:errcheck // a clean exit leaves nothing to keep
		return false
	}
	name := crashFilePrefix + info.ModTime().UTC().Format("20060102-150405") + "-" +
		strings.TrimSuffix(filepath.Base(path), crashFileSuffix) + crashFileSuffix
	return os.Rename(path, filepath.Join(dir, name)) == nil
}

// holdsRuntimeOutput answers whether the runtime wrote anything into a pending
// file. A file the header write never completed holds no marker, and whatever it
// holds is kept rather than guessed at.
func holdsRuntimeOutput(data []byte) bool {
	i := bytes.Index(data, []byte(runtimeOutputMarker))
	if i < 0 {
		return len(data) > 0
	}
	return i+len(runtimeOutputMarker) < len(data)
}
