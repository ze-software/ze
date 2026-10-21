// Design: docs/architecture/testing/qemu-integration.md -- a VM never outlives its le
// Related: run_exec.go -- executePlan writes the record and reaps before it boots
// Related: run_procattr_linux.go -- Pdeathsig, the Linux half of the same guarantee
//
// A run's QEMU leads a process group of its own, so a terminal's Ctrl-C does
// not end it under the guest command. The price is that a le killed outright
// (SIGKILL, a crash) leaves its VM running. Linux closes that with Pdeathsig.
// macOS has no such attribute, so a run records its QEMU here, and the next
// run stops a QEMU whose le is gone.
package testqemu

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// errProcessGone is what StartTime answers for a pid no process holds.
var errProcessGone = errors.New("no such process")

// vmRecord names a run's QEMU and the le that started it, each with its start
// time, so a reused pid is never taken for the recorded process.
type vmRecord struct {
	QEMUPid   int   `json:"qemu-pid"`
	QEMUStart int64 `json:"qemu-start"`
	LePid     int   `json:"le-pid"`
	LeStart   int64 `json:"le-start"`
}

// vmRecordPrefix starts the name of every VM record, one per running VM of
// this checkout. The records sit in tmp/qemu itself rather than a directory of
// their own, so a clean stop leaves nothing behind there.
const vmRecordPrefix = "vm-record-"

// vmRecordStagedPrefix starts the name a record is written under before its
// rename into place (recordVM).
const vmRecordStagedPrefix = ".vm-record-staged-"

// vmRecordUnreadableAge is how long an unreadable record is kept before a
// reap drops it. A record is published whole by a rename, so an unreadable
// one is a damaged file rather than one being written; the age is the margin
// for a filesystem that does not rename atomically, and it costs nothing but
// a line on each run that meets it.
const vmRecordUnreadableAge = 10 * time.Minute

// vmRecordDir is the directory that holds the records.
func (r *Run) vmRecordDir() string { return filepath.Join(r.Tree, "tmp", "qemu") }

func vmRecordName(pid int) string { return vmRecordPrefix + strconv.Itoa(pid) + ".json" }

// recordVM writes the record of the QEMU just started as qemuPid. The caller
// MUST call the answered remove once the VM is stopped.
func (r *Run) recordVM(qemuPid int) (func(), error) {
	qemuStart, err := r.ops.StartTime(qemuPid)
	if err != nil {
		return nil, fmt.Errorf("read the start time of QEMU pid %d: %w", qemuPid, err)
	}
	lePid := os.Getpid()
	leStart, err := r.ops.StartTime(lePid)
	if err != nil {
		return nil, fmt.Errorf("read the start time of le pid %d: %w", lePid, err)
	}
	data, err := json.Marshal(vmRecord{QEMUPid: qemuPid, QEMUStart: qemuStart, LePid: lePid, LeStart: leStart})
	if err != nil {
		return nil, fmt.Errorf("encode the VM record: %w", err)
	}
	if err := os.MkdirAll(r.vmRecordDir(), 0o750); err != nil {
		return nil, fmt.Errorf("create the VM record directory: %w", err)
	}
	path := filepath.Join(r.vmRecordDir(), vmRecordName(qemuPid))
	// Written aside, then renamed into place, so a concurrent reap never
	// reads half a record: the temporary name carries no record prefix.
	staged, err := os.CreateTemp(r.vmRecordDir(), vmRecordStagedPrefix+"*")
	if err != nil {
		return nil, fmt.Errorf("create the VM record: %w", err)
	}
	stagedName := staged.Name()
	_, err = staged.Write(data)
	if closeErr := staged.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(stagedName, path)
	}
	if err != nil {
		os.Remove(stagedName) //nolint:errcheck // a staged file names no VM, so nothing reads it
		return nil, fmt.Errorf("write the VM record: %w", err)
	}
	return func() { os.Remove(path) }, nil //nolint:errcheck // a record left behind is dropped by the next reap
}

// reapOrphanVMs reads every record and stops a QEMU only when both hold: the
// le that recorded it is gone (no process at its pid, or one that started at
// another time), and the QEMU pid still names the process recorded. A record
// whose le still runs is another session's live VM and is left alone. A record
// whose QEMU is gone or whose pid was reused is dropped. A record nobody can
// judge is kept and named. Each action is one line on log.
func (r *Run) reapOrphanVMs(log io.Writer) {
	entries, err := os.ReadDir(r.vmRecordDir())
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			reapNote(log, "could not read the VM records, so no VM left by a dead le was looked for: "+err.Error())
		}
		return
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), vmRecordStagedPrefix) {
			removeStaleStaged(filepath.Join(r.vmRecordDir(), entry.Name()))
			continue
		}
		if !strings.HasPrefix(entry.Name(), vmRecordPrefix) || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		r.reapOne(log, filepath.Join(r.vmRecordDir(), entry.Name()))
	}
}

// reapOne judges one record (reapOrphanVMs).
func (r *Run) reapOne(log io.Writer, path string) {
	data, err := os.ReadFile(path) // #nosec G304 -- a file of this run's own record directory
	if err != nil {
		reapNote(log, "could not read the VM record "+path+": "+err.Error())
		return
	}
	var record vmRecord
	if err := json.Unmarshal(data, &record); err != nil {
		r.reapUnreadable(log, path, err)
		return
	}
	leStart, err := r.ops.StartTime(record.LePid)
	leGone := errors.Is(err, errProcessGone)
	if err != nil && !leGone {
		reapNote(log, "kept the VM record "+path+": could not tell whether le pid "+strconv.Itoa(record.LePid)+" still runs: "+err.Error())
		return
	}
	if !leGone && leStart == record.LeStart {
		return // another le's live VM
	}
	qemuStart, err := r.ops.StartTime(record.QEMUPid)
	if errors.Is(err, errProcessGone) {
		os.Remove(path) //nolint:errcheck // the VM it named has already stopped
		return
	}
	if err != nil {
		reapNote(log, "kept the VM record "+path+": could not read QEMU pid "+strconv.Itoa(record.QEMUPid)+": "+err.Error())
		return
	}
	if qemuStart != record.QEMUStart {
		os.Remove(path) //nolint:errcheck // the pid now names another process, which is never touched
		return
	}
	if err := r.ops.Kill(record.QEMUPid); err != nil {
		reapNote(log, "could not stop QEMU pid "+strconv.Itoa(record.QEMUPid)+", left running by le pid "+
			strconv.Itoa(record.LePid)+", which is gone: "+err.Error()+"\n  stop it with: kill "+strconv.Itoa(record.QEMUPid))
		return
	}
	os.Remove(path) //nolint:errcheck // the VM it named is stopped
	reapNote(log, "stopped QEMU pid "+strconv.Itoa(record.QEMUPid)+": le pid "+strconv.Itoa(record.LePid)+
		", which started it, is gone, so nothing else would have stopped it")
}

// reapUnreadable keeps an unreadable record younger than
// vmRecordUnreadableAge and names it, and drops an older one: it names no
// process, so nothing can act on it.
func (r *Run) reapUnreadable(log io.Writer, path string, cause error) {
	info, err := os.Stat(path)
	if err != nil {
		reapNote(log, "could not read the VM record "+path+": "+err.Error())
		return
	}
	if time.Since(info.ModTime()) < vmRecordUnreadableAge {
		reapNote(log, "kept the unreadable VM record "+path+" ("+cause.Error()+"): it is younger than "+
			vmRecordUnreadableAge.String()+", so another run may still be writing it")
		return
	}
	reapNote(log, "dropped the unreadable VM record "+path+": "+cause.Error())
	os.Remove(path) //nolint:errcheck // it names no process, so nothing can act on it
}

// removeStaleStaged removes a staged record older than vmRecordUnreadableAge:
// a le killed between staging and rename left it, and it names no VM. A
// younger one may be a live run's, about to be renamed, and is kept.
func removeStaleStaged(path string) {
	info, err := os.Stat(path)
	if err != nil {
		return // already renamed or removed
	}
	if time.Since(info.ModTime()) < vmRecordUnreadableAge {
		return
	}
	os.Remove(path) //nolint:errcheck // nothing reads a staged record, so one left over costs nothing
}

func reapNote(log io.Writer, note string) {
	var b textbuf.Buffer
	log.Write(b.Str("  ").Str(note).Byte('\n').Bytes()) //nolint:errcheck // progress output
}

// killProcess is the production Kill: SIGKILL, because the le that would have
// stopped the VM gently is gone and nobody waits for the guest.
func killProcess(pid int) error { return syscall.Kill(pid, syscall.SIGKILL) }
