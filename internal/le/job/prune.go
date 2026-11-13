// Design: docs/contributing/running-commands.md -- waiting for another session's job, and "When the disk is full"
// Overview: registry.go -- the files this removes, and the lock it takes
// Related: attach.go -- the follower whose need for a receipt bounds the age
// Related: waiter.go -- the waiter records
// Related: overview.go -- the one process-table scan this judges liveness with
// Related: ../scratch/cacheclean.go -- scratch clean, the caller
//
// The registry kept every run's result receipt for ever, and a killed job
// left its entry, its log and its waiter record for ever too: on 2026-11-13
// the directory held 4,756 receipts beside 13 entries
// (plan/journal/artifact-written-with-no-remover.md). A job and a waiter
// still never remove a file another process wrote. The remover is the
// checkout's cleaner, `le scratch clean`, and this file is the judgment it
// calls: which registry files are dead, by the registry's own formats, and
// old enough that nothing can still read them.

package job

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/le/procscan"
)

// PruneAgeMin is the least age bound Prune accepts: how long a follower can
// still read a receipt after its holder wrote it. A follower notices the
// entry is gone within one poll and one process query (follow, attach.go),
// and then reads the receipt for the result wait (attach). Prune refuses a
// smaller bound, because a receipt removed inside that window turns a shared
// verdict into a missing one.
const PruneAgeMin = PollDefault + aliveTimeout + ResultWaitDefault

// PruneReason says why Prune removed a registry file or kept it. The zero
// value is no verdict.
type PruneReason uint8

const (
	// PruneUnspecified is the zero value and is never a verdict.
	PruneUnspecified PruneReason = iota
	// PruneRemovedDeadEntry is an entry whose process is not running.
	PruneRemovedDeadEntry
	// PruneRemovedDeadEntryLog is the log of such an entry.
	PruneRemovedDeadEntryLog
	// PruneRemovedReceipt is a result receipt whose entry is gone.
	PruneRemovedReceipt
	// PruneRemovedOrphanLog is a log whose entry is gone: a holder that died
	// between removing its entry and removing its log left it.
	PruneRemovedOrphanLog
	// PruneRemovedDeadWaiter is a waiter record whose process is not running.
	PruneRemovedDeadWaiter
	// PruneKeptYoung is a dead file younger than the age bound.
	PruneKeptYoung
	// PruneKeptRunning is an entry or a waiter record whose process runs.
	PruneKeptRunning
	// PruneKeptLivenessUnknown is a file whose process nobody could judge,
	// because the process table could not be read.
	PruneKeptLivenessUnknown
	// PruneKeptUnreadable is a file whose fields or state could not be read,
	// so it names no process to judge.
	PruneKeptUnreadable
	// PruneKeptEntryPresent is a receipt or a log whose entry is still there.
	PruneKeptEntryPresent
	// PruneKeptForeignName is a file whose name is not one the registry
	// writes, or that is not a plain file.
	PruneKeptForeignName
	// PruneKeptRemoveFailed is a dead file the removal failed on.
	PruneKeptRemoveFailed
)

// String answers the reason as the words a report prints after a count of
// files. An unspecified reason is a Ze defect.
func (r PruneReason) String() string {
	switch r {
	case PruneRemovedDeadEntry:
		return "entries of a process that is not running"
	case PruneRemovedDeadEntryLog:
		return "logs of a process that is not running"
	case PruneRemovedReceipt:
		return "result receipts whose entry is gone"
	case PruneRemovedOrphanLog:
		return "logs whose entry is gone"
	case PruneRemovedDeadWaiter:
		return "waiter records of a process that is not running"
	case PruneKeptYoung:
		return "younger than the age bound"
	case PruneKeptRunning:
		return "of a running process"
	case PruneKeptLivenessUnknown:
		return "of a process nobody could judge"
	case PruneKeptUnreadable:
		return "unreadable"
	case PruneKeptEntryPresent:
		return "beside an entry that is still there"
	case PruneKeptForeignName:
		return "not named as the registry names its files"
	case PruneKeptRemoveFailed:
		return "not removable"
	case PruneUnspecified:
		panic("BUG: a registry prune row carries no reason")
	default:
		panic("BUG: unknown registry prune reason")
	}
}

// PruneRow is one registry file Prune judged.
type PruneRow struct {
	// Path is the file, absolute.
	Path string
	// Bytes is the file's length.
	Bytes int64
	// Reason is why the file was removed or kept.
	Reason PruneReason
	// Err is the failure behind an unreadable or unremovable file, and nil
	// for every other reason.
	Err error
}

// PruneReport is what one Prune did: every file it removed and every file it
// kept, each with its reason, in name order with the entries first.
type PruneReport struct {
	// Absent says the checkout has no registry directory, so there was
	// nothing to judge. It is not an empty registry.
	Absent  bool
	Removed []PruneRow
	Kept    []PruneRow
}

// PrunePolicy is what a prune reads besides the registry: the clock, the age
// bound and the process scanner. A test replaces each one.
type PrunePolicy struct {
	// Now is the instant ages are measured from.
	Now time.Time
	// Age is the bound a dead file must be older than. It MUST be at least
	// PruneAgeMin.
	Age time.Duration
	// Scan reads the process table. It is called once.
	Scan func() ([]procscan.Process, error)
}

// Prune removes the dead files of the job registry of the checkout at root
// that are older than the policy's age bound, and answers what it removed and
// what it kept.
//
// A file is removed only when the registry's own formats call it dead:
//
//   - an entry whose fields name a process the scan does not list, with the
//     log beside it. Its age is that of the newer of the two files, so a job
//     whose log is still growing is never old, whatever the scan said.
//   - a result receipt, or a log, whose entry is gone. Only a holder removes
//     its entry, at Release or because it died later than that, so the job
//     has ended.
//   - a waiter record whose fields name a process the scan does not list.
//
// Everything else stays: a file younger than the bound, a process that runs,
// a process nobody could judge because the scan failed, a file whose fields
// cannot be read, a name the registry does not write, and the lock.
//
// The caller MUST be a cleaner an operator started (`le scratch clean`), and
// MUST NOT be a job or a waiter: admission never removes a file another
// process wrote (scanAndClaim). The age bound MUST be at least PruneAgeMin
// and Prune refuses a smaller one. The other side of each pair:
//
//   - A follower (attach) MUST read its holder's receipt within PruneAgeMin
//     of the holder writing it, which its poll and its result wait already
//     bound. One stopped for longer than the age bound finds no receipt,
//     reports nothing observed and runs the job itself: a repeated run,
//     never a wrong verdict.
//   - writeResult and take MUST keep creating their files exclusively under
//     a name carrying a fresh claim. A file they create is younger than the
//     bound, so Prune never removes it, and no new file takes the name of a
//     dead one while Prune judges it.
//   - The scan is one reading of the process table taken before the files
//     are listed. A process that started after it is not listed and reads as
//     gone, and every file it wrote is younger than the bound, which is why
//     the bound and not the scan alone decides.
//
// Prune holds the registry lock from its listing to its last removal. The
// lock is not what protects a follower or a new claim: the age bound and the
// unique names do. It is held because an admission scan counts an entry that
// vanishes between its listing and its read as occupied (scanAndClaim fails
// closed), so a scan running through a prune would see removed dead entries
// as held slots and make its job wait. The process table is read before the
// lock is taken, so no admission waits behind a slow `ps`.
//
// An absent registry answers a report that says so. A registry that cannot be
// listed, or whose lock cannot be taken, answers an error and removes nothing.
func Prune(ctx context.Context, root string, policy PrunePolicy) (PruneReport, error) {
	if policy.Age < PruneAgeMin {
		return PruneReport{}, fmt.Errorf("the job registry prune was asked for an age bound of %s, below the %s a follower may still read a receipt for", policy.Age, PruneAgeMin)
	}
	if policy.Scan == nil {
		panic("BUG: the job registry prune was given no process scanner")
	}

	registry := &Admission{Root: root}
	var report PruneReport
	if _, err := os.Stat(registry.jobsDir()); errors.Is(err, fs.ErrNotExist) {
		report.Absent = true
		return report, nil
	}

	table := scanTable(policy.Scan)

	lock, err := registry.lockRegistry(ctx)
	if err != nil {
		return report, fmt.Errorf("take the job registry lock: %w", err)
	}
	defer lock.release(registry)

	listed, err := os.ReadDir(registry.jobsDir())
	if err != nil {
		return report, fmt.Errorf("list the job registry %s: %w", JobsDir, err)
	}

	pass := prunePass{table: table, now: policy.Now, age: policy.Age, report: &report, gone: map[string]struct{}{}}
	files := pass.registryFiles(registry.jobsDir(), listed)

	// Entries first: a dead entry takes its log with it, and its receipt then
	// has no entry beside it when the second round reaches it.
	for _, file := range files {
		if file.suffix == entrySuffix {
			pass.entry(registry, file)
		}
	}
	for _, file := range files {
		if _, removed := pass.gone[file.path]; removed {
			continue
		}
		if file.suffix == resultSuffix {
			pass.orphan(file, PruneRemovedReceipt)
		}
		if file.suffix == logSuffix {
			pass.orphan(file, PruneRemovedOrphanLog)
		}
		if file.suffix == waiterSuffix {
			pass.waiter(file)
		}
	}
	return report, nil
}

// registrySuffixes are the four suffixes the registry writes (registry.go,
// waiter.go).
var registrySuffixes = []string{entrySuffix, logSuffix, resultSuffix, waiterSuffix}

// registryFile is one file of the registry directory whose name the registry
// writes.
type registryFile struct {
	path   string
	suffix string
	bytes  int64
	// modified is when the file was last written.
	modified time.Time
}

// prunePass is the state of one Prune.
type prunePass struct {
	table  processTable
	now    time.Time
	age    time.Duration
	report *PruneReport
	// gone holds the paths this pass removed, so the second round does not
	// judge a log the first round took with its entry.
	gone map[string]struct{}
}

// registryFiles answers the listed files the registry itself names, and
// records every other one as kept. The lock is neither: it is a lock and not
// a record, and it stays.
func (p *prunePass) registryFiles(dir string, listed []fs.DirEntry) []registryFile {
	files := make([]registryFile, 0, len(listed))
	for _, listing := range listed {
		name := listing.Name()
		if name == lockName {
			continue
		}
		file := registryFile{path: filepath.Join(dir, name)}
		suffix, shaped := registryShape(name)
		if !shaped {
			p.keep(file, PruneKeptForeignName, nil)
			continue
		}
		if !listing.Type().IsRegular() {
			// A link under a registry name could point outside the registry.
			p.keep(file, PruneKeptForeignName, nil)
			continue
		}
		info, err := listing.Info()
		if errors.Is(err, fs.ErrNotExist) {
			// Removed since the listing: a holder releases without the lock,
			// and this one has just ended.
			continue
		}
		if err != nil {
			p.keep(file, PruneKeptUnreadable, err)
			continue
		}
		file.suffix = suffix
		file.bytes = info.Size()
		file.modified = info.ModTime()
		files = append(files, file)
	}
	return files
}

// registryShape answers the suffix of a name the registry writes,
// <label>.<pid>.<claim><suffix>, and whether the name is one.
func registryShape(name string) (string, bool) {
	for _, suffix := range registrySuffixes {
		stem, found := strings.CutSuffix(name, suffix)
		if !found {
			continue
		}
		return suffix, claimStem(stem)
	}
	return "", false
}

// claimStem reports whether a stem is <label>.<pid>.<claim>: a label, a
// positive process id in decimal, and the claim an exclusive creation chose.
func claimStem(stem string) bool {
	parts := strings.Split(stem, ".")
	if len(parts) != 3 {
		return false
	}
	if !validLabel(parts[0]) {
		return false
	}
	pid, err := strconv.Atoi(parts[1])
	if err != nil {
		return false
	}
	if pid <= 0 {
		return false
	}
	if strconv.Itoa(pid) != parts[1] {
		return false
	}
	// A claim is a path component like a label, so the same letters bound it.
	return validLabel(parts[2])
}

// entry judges one entry and removes it, with its log, when its process is
// gone and neither file was written inside the age bound.
func (p *prunePass) entry(registry *Admission, file registryFile) {
	held, readable := registry.readEntry(file.path)
	if !readable {
		p.keep(file, PruneKeptUnreadable, nil)
		return
	}
	if p.keptAlive(file, held.pid) {
		return
	}

	// The log is the sibling take created, never the path the LOG field
	// names: that field is another process's data.
	log := registryFile{path: swapSuffix(file.path, logSuffix)}
	newest := file.modified
	logInfo, err := os.Lstat(log.path)
	logPresent := err == nil
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		p.keep(file, PruneKeptUnreadable, err)
		return
	}
	if logPresent {
		if !logInfo.Mode().IsRegular() {
			p.keep(file, PruneKeptUnreadable, nil)
			return
		}
		log.bytes = logInfo.Size()
		log.modified = logInfo.ModTime()
		if log.modified.After(newest) {
			newest = log.modified
		}
	}
	if !p.old(newest) {
		p.keep(file, PruneKeptYoung, nil)
		return
	}

	// The entry goes before the log, as Release orders them: a prune that
	// stops between the two leaves a log with no entry, which the next one
	// removes.
	if !p.drop(file, PruneRemovedDeadEntry) {
		return
	}
	if logPresent {
		p.drop(log, PruneRemovedDeadEntryLog)
	}
}

// orphan judges one receipt or log, and removes it when its entry is gone and
// it is older than the age bound.
func (p *prunePass) orphan(file registryFile, removed PruneReason) {
	entryPath := strings.TrimSuffix(file.path, file.suffix) + entrySuffix
	_, err := os.Lstat(entryPath)
	if err == nil {
		p.keep(file, PruneKeptEntryPresent, nil)
		return
	}
	if !errors.Is(err, fs.ErrNotExist) {
		p.keep(file, PruneKeptUnreadable, err)
		return
	}
	if !p.old(file.modified) {
		p.keep(file, PruneKeptYoung, nil)
		return
	}
	p.drop(file, removed)
}

// waiter judges one waiter record, and removes it when its process is gone
// and it is older than the age bound.
func (p *prunePass) waiter(file registryFile) {
	waiting, readable := readWaiter(file.path)
	if !readable {
		p.keep(file, PruneKeptUnreadable, nil)
		return
	}
	if p.keptAlive(file, waiting.pid) {
		return
	}
	if !p.old(file.modified) {
		p.keep(file, PruneKeptYoung, nil)
		return
	}
	p.drop(file, PruneRemovedDeadWaiter)
}

// keptAlive records a file as kept when the scan does not show its process
// gone, and reports whether it did.
func (p *prunePass) keptAlive(file registryFile, pid int) bool {
	switch p.table.judge(pid) {
	case livenessRunning:
		p.keep(file, PruneKeptRunning, nil)
		return true
	case livenessUnknown:
		p.keep(file, PruneKeptLivenessUnknown, p.table.err)
		return true
	case livenessGone:
		return false
	case livenessUnspecified:
		panic("BUG: the process table answered no verdict for a registry file")
	default:
		panic("BUG: unknown registry file liveness")
	}
}

// old reports whether a file last written at modified is older than the age
// bound. A time after now is not old.
func (p *prunePass) old(modified time.Time) bool {
	return p.now.Sub(modified) > p.age
}

// keep records a file that stays.
func (p *prunePass) keep(file registryFile, reason PruneReason, err error) {
	p.report.Kept = append(p.report.Kept, PruneRow{Path: file.path, Bytes: file.bytes, Reason: reason, Err: err})
}

// drop removes one dead file and records it, and reports whether the file is
// gone. A failure is recorded as a kept file and the file stays. A file that
// was already gone is not counted as removed: this pass did not remove it.
func (p *prunePass) drop(file registryFile, reason PruneReason) bool {
	err := os.Remove(file.path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		p.keep(file, PruneKeptRemoveFailed, err)
		return false
	}
	p.gone[file.path] = struct{}{}
	if err == nil {
		p.report.Removed = append(p.report.Removed, PruneRow{Path: file.path, Bytes: file.bytes, Reason: reason})
	}
	return true
}
