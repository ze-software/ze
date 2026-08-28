// Design: docs/architecture/core-design.md -- the registry that IS the claim
// Detail: treehash.go -- the tree an entry records
// Detail: jobkey.go -- the work an entry records
//
// tmp/.ze-jobs/<label>.<pid>.job contains one FIELD=VALUE line per field, and
// its PRESENCE is the claim on a slot. No separate record identifies the
// running job. The holder removes its entry when it exits. A waiter removes an
// entry if its process is gone. Thus, a crashed job causes a delay of one poll
// interval and does not require an operator.
//
// The format matches internal/le/job/answer.go field for field and file name for
// file name. Both implementations read and write this directory during the
// migration. If a field is renamed here, the other implementation cannot see
// that job.
//
// The initial alternative used plain flock over more targets and no registry.
// It provides no method to identify the slot holder, remove a crashed holder
// without an operator, or record the tree hash. The tree hash later lets a
// second asker share the result of a running job.

package job

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// aliveTimeout limits the process query for another job. One second for `ps`
// is already abnormal. The timeout prevents a wedged `ps` process from holding
// the registry lock.
const aliveTimeout = 5 * time.Second

// killGrace is how long a stalled holder is given to die on a TERM before it
// is killed outright.
const killGrace = 3 * time.Second

// One job owns three suffixes in the registry. They identify the entry that IS
// its claim, the log that supplies liveness, and the result that holds a verdict.
const (
	entrySuffix  = ".job"
	logSuffix    = ".log"
	resultSuffix = ".rc"
)

// state is what the registry answered a job asking to run.
type state uint8

const (
	// stateBusy is intentionally the zero value. If a scan ends without a
	// decision, the asker waits and does not run. Thus, the package fails
	// closed.
	stateBusy state = iota
	// stateClaimed is a slot taken, with this job's entry written.
	stateClaimed
	// stateAttach is an identical job already running, to be followed.
	stateAttach
)

// pending is one job asking to run. It carries what the registry is asked
// about and what would be written if the answer is yes.
type pending struct {
	label     string
	argv      []string
	tree      string
	key       string
	mayAttach bool
	// treeStale indicates that this job's tree hash predates its admission
	// because the job waited. The tree can change while a job waits. Therefore,
	// the hash is calculated again at admission, but only for a job that waited.
	treeStale bool
}

// outcome is what one scan decided.
type outcome struct {
	state state
	// entry is the registry entry, relative to the root: this job's when the
	// state is claimed, the holder's when it is attach.
	entry string
	// log is that entry's log, relative to the root.
	log string
	// pid is the holder's process, when the state is attach or busy.
	pid int
	// holder names what is occupying a slot, for the waiting banner.
	holder  string
	elapsed time.Duration
	started time.Time
}

// entry is one registry file, parsed.
type entry struct {
	// path is the absolute path of the entry file.
	path  string
	label string
	pid   int
	pgid  int
	tree  string
	key   string
	// log is the entry's LOG field as written: relative to the root.
	log     string
	state   string
	started time.Time
}

// claim asks the registry once, under the registry lock, and answers what it
// decided.
func (a *Admission) claim(job *pending) outcome {
	lock, err := a.lockRegistry()
	if err != nil {
		// The process failed to inspect the registry, so it does not admit a job.
		return outcome{state: stateBusy, holder: "registry lock"}
	}
	defer lock.release(a)

	return a.scanAndClaim(job)
}

// scanAndClaim removes dead entries, counts the live entries, and then shares
// an equivalent job, claims a slot, or reports the holder.
//
// MUST run with the registry lock. It counts directory entries and then writes
// an entry. If two scans overlap, both detect the same free slot.
func (a *Admission) scanAndClaim(job *pending) outcome {
	now := time.Now()
	occupied := 0
	result := outcome{state: stateBusy, holder: "?"}
	var share *entry

	names, err := filepath.Glob(filepath.Join(a.jobsDir(), "*"+entrySuffix))
	if err != nil {
		// A directory that cannot even be listed is a registry that cannot be
		// judged, so nothing is admitted against it.
		return outcome{state: stateBusy, holder: "unreadable registry"}
	}
	slices.Sort(names)

	for _, path := range names {
		held, ok := a.readEntry(path)
		if !ok {
			// FAIL CLOSED: an entry that cannot be read is a job that cannot
			// be proven gone. It has no readable LOG to judge, so this one
			// case still goes by age, and it only DROPS the entry -- nothing
			// is signaled. The window is shared so the registry stays bounded
			// by one number.
			age := now.Sub(fileTime(path))
			if age > a.Stall {
				a.reap(path)
				continue
			}
			occupied++
			if result.holder == "?" {
				var tb textbuf.Buffer
				result.holder = tb.Str("unreadable entry ").Str(filepath.Base(path)).String()
				result.elapsed = age
			}
			continue
		}

		if !alive(held.pid) {
			a.reap(path)
			continue
		}

		elapsed := now.Sub(held.started)

		// The holder is alive. Its log, not the clock, determines whether it is
		// WORKING. Each write updates the file's mtime. Thus, a log that grew
		// during the stall window identifies a job that is still producing
		// output. If the log is unreadable or absent, the holder keeps its slot.
		//
		// The system cannot assess that holder. Admission of a second job would
		// oversubscribe the box, and a kill without evidence would destroy a run.
		if written := fileTime(a.abs(held.log)); held.log != "" && !written.IsZero() {
			static := now.Sub(written)
			if static > a.Stall {
				a.breakStalled(held, static, elapsed)
				continue
			}
		}

		if share == nil && a.shares(job, held) {
			share = &held
		}

		occupied++
		if result.holder == "?" {
			result.holder = held.label
			result.pid = held.pid
			result.elapsed = elapsed
		}
	}

	a.retireResults(now)

	if share != nil {
		return outcome{state: stateAttach, entry: a.rel(share.path), log: share.log, pid: share.pid}
	}
	if occupied >= a.Slots {
		return result
	}
	return a.take(job, now)
}

// shares reports whether a running job performs the asker's work on the
// asker's tree. Both conditions must be true before the asker uses its verdict.
//
// A value that cannot be measured matches no value. It also does not match an
// unmeasured value from another job. An unmeasured tree is not a matching tree,
// and an unmeasured key is not a matching key.
func (a *Admission) shares(job *pending, held entry) bool {
	if !job.mayAttach || held.state != "running" || held.label != job.label {
		return false
	}
	if job.tree == "" || job.tree == Unknown || held.tree != job.tree {
		return false
	}
	if job.key == "" || job.key == Unknown || held.key != job.key {
		return false
	}
	return true
}

// take writes this job's entry, which IS its claim on a slot.
func (a *Admission) take(job *pending, now time.Time) outcome {
	// A later asker uses TREE to attach, so TREE must identify the tree that this
	// job will judge. A job can wait behind a twenty minute holder. During that
	// wait, the tree that the job originally requested can change.
	if job.treeStale {
		job.tree = TreeHash(a.Root)
	}

	pid := os.Getpid()
	rel := filepath.Join(JobsDir, entryName(job.label, pid, entrySuffix))
	logRel := filepath.Join(JobsDir, entryName(job.label, pid, logSuffix))

	var tb textbuf.Buffer
	tb.Str("LABEL=").Str(job.label).Byte('\n')
	tb.Str("PID=").Int(int64(pid)).Byte('\n')
	tb.Str("PGID=").Int(int64(processGroup())).Byte('\n')
	tb.Str("TREE=").Str(job.tree).Byte('\n')
	// KEY determines whether a later asker can share this run.
	tb.Str("KEY=").Str(job.key).Byte('\n')
	tb.Str("STARTED=").Int(now.Unix()).Byte('\n')
	tb.Str("LOG=").Str(logRel).Byte('\n')
	tb.Str("STATE=running\n")
	tb.Str("CMD=").Join(job.argv, " ").Byte('\n')
	written := []byte(tb.String())

	if err := os.WriteFile(a.abs(rel), written, 0o644); err != nil { //nolint:gosec // the registry is a shared checkout's tmp/, read by every session on this machine
		a.note(errorLine(err))
		return outcome{state: stateBusy, holder: "unwritable registry"}
	}
	if err := os.WriteFile(a.abs(logRel), nil, 0o644); err != nil { //nolint:gosec // same registry, same readers
		a.note(errorLine(err))
	}
	if err := os.WriteFile(a.abs(OwnerFile), written, 0o644); err != nil { //nolint:gosec // the documented view of the holder, which ai/rules tells readers to open
		a.note(errorLine(err))
	}

	return outcome{state: stateClaimed, entry: rel, log: logRel, pid: pid, started: now}
}

// breakStalled kills a live holder that stopped producing output and then
// removes its entry.
//
// The kill message includes the evidence that justified the kill. It identifies
// the file that stopped growing and the duration without growth. Elapsed time
// is context and is never the reason. An operator who reads
// "20 minutes elapsed" cannot determine if the kill was correct. One who reads
// "no output for 31 minutes" can.
//
// This method replaced a decision based on age. A holder older than 1800
// seconds previously had its process group killed. The threshold was justified
// by "verify targets ~2 min", but recorded history gives 12m45s. A full run
// under load on 2026-08-17 took more than 20 minutes. Under the contention that
// this package manages, the first waiter killed a valid run. The waiter then
// ran slowly, and the next waiter killed it.
func (a *Admission) breakStalled(held entry, static, elapsed time.Duration) {
	target, ok := signalTarget(held)
	if !ok {
		// Nothing safe to signal: leave the entry alone rather than kill this
		// session's own process group.
		return
	}

	colors := textbuf.C
	var tb textbuf.Buffer
	tb.SetColor(a.Color)
	tb.Colored(colors.BoldRed).Byte('[').Str(held.label).Str("] breaking STALLED job: pid ").
		Int(int64(held.pid)).Str(", pgid ").Int(int64(held.pgid)).Colored(colors.Reset)
	a.note(tb.String())

	tb.Reset()
	tb.SetColor(a.Color)
	tb.Colored(colors.BoldRed).Str("  evidence: ").Str(held.log).
		Str(" has not grown for ").Int(int64(static / time.Second)).
		Str("s (stall window ").Int(int64(a.Stall / time.Second)).
		Str("s); the job had been running ").Int(int64(elapsed / time.Second)).Str("s").
		Colored(colors.Reset)
	a.note(tb.String())

	if err := syscall.Kill(target, syscall.SIGTERM); err != nil && !errors.Is(err, syscall.ESRCH) {
		a.note(errorLine(err))
	}
	deadline := time.Now().Add(killGrace)
	for alive(held.pid) && time.Now().Before(deadline) {
		time.Sleep(time.Second)
	}
	if err := syscall.Kill(target, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
		a.note(errorLine(err))
	}

	a.reap(held.path)
}

// signalTarget returns the target for the signal that stops a stalled holder.
// It also reports whether the target can be signaled safely.
//
// The process GROUP is the target because a job is a process tree. If only the
// leader is killed, the remaining processes continue to use the machine. Two
// targets are never signaled: process group 0, which includes every process in
// the session, and this process's own group.
func signalTarget(held entry) (int, bool) {
	if held.pgid > 0 && held.pgid != processGroup() {
		return -held.pgid, true
	}
	if held.pid > 0 && held.pid != os.Getpid() {
		return held.pid, true
	}
	return 0, false
}

// retireResults removes results for jobs whose attachers did not return.
//
// An attacher reads a result shortly after it is written. No operation deletes
// the result for its reader, so age limits this part of the registry. Results
// use the same window as entries.
func (a *Admission) retireResults(now time.Time) {
	names, err := filepath.Glob(filepath.Join(a.jobsDir(), "*"+resultSuffix))
	if err != nil {
		return
	}
	for _, path := range names {
		if now.Sub(fileTime(path)) > a.Stall {
			a.remove(path)
		}
	}
}

// writeResult records a finished job's verdict where a follower can find it.
//
// There is no default and no zero: a job killed outright leaves no record, and
// the attacher then reports nothing at all. It returns to the admission queue
// and runs the job itself, because a verdict it did not observe is worse than
// the work it avoided.
func (a *Admission) writeResult(entryRel string, code int) {
	var tb textbuf.Buffer
	body := tb.Int(int64(code)).Byte('\n').String()

	if err := os.WriteFile(a.abs(swapSuffix(entryRel, resultSuffix)), []byte(body), 0o644); err != nil { //nolint:gosec // the registry is a shared checkout's tmp/, read by every session on this machine
		a.note(errorLine(err))
	}
}

// readResult answers the verdict a finished job recorded, and reports whether
// there was one.
func (a *Admission) readResult(entryRel string) (int, bool) {
	body, err := os.ReadFile(a.abs(swapSuffix(entryRel, resultSuffix))) //nolint:gosec // a registry path this package built from a validated label
	if err != nil {
		return 0, false
	}
	code, err := strconv.Atoi(string(trimNewline(body)))
	if err != nil {
		// An unreadable result is a failure, never a pass: the shared run's
		// verdict is the whole point of following it.
		return 1, true
	}
	return code, true
}

// appendDuration records how long one job took, which is what lets the next
// asker for the same label be told what it is waiting for.
func (a *Admission) appendDuration(label string, took time.Duration) {
	var tb textbuf.Buffer
	line := tb.Str(label).Byte('\t').Int(int64(took / time.Second)).Byte('\t').
		Str(time.Now().UTC().Format("2006-01-02T15:04:05Z")).Byte('\n').String()

	file, err := os.OpenFile(a.abs(DurationFile), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644) //nolint:gosec // a fixed path under the checkout's tmp/
	if err != nil {
		return
	}
	if _, err := file.WriteString(line); err != nil {
		a.note(errorLine(err))
	}
	if err := file.Close(); err != nil {
		a.note(errorLine(err))
	}
}

// reportPrevious tells a job what the last run of its label cost, which is the
// one number that says whether waiting is worth it.
func (a *Admission) reportPrevious(label string) {
	body, err := os.ReadFile(a.abs(DurationFile)) //nolint:gosec // a fixed path under the checkout's tmp/
	if err != nil {
		return
	}

	seconds := ""
	for line := range strings.SplitSeq(string(body), "\n") {
		fields := strings.Split(line, "\t")
		if len(fields) >= 2 && fields[0] == label {
			seconds = fields[1]
		}
	}
	took, err := strconv.Atoi(seconds)
	if err != nil {
		return
	}

	var tb textbuf.Buffer
	a.note(tb.Byte('[').Str(label).Str("] previous run took ").
		Int(int64(took / 60)).Str("m").Int(int64(took % 60)).Str("s (").
		Int(int64(took)).Str("s)").String())
}

// reportBusy writes the banner a waiting job repeats while it waits.
func (a *Admission) reportBusy(label string, result outcome) {
	colors := textbuf.C
	var tb textbuf.Buffer
	tb.SetColor(a.Color)
	tb.Colored(colors.BrightYellow).Byte('[').Str(label).Str("] waiting: ").Str(result.holder).
		Str(" running (pid ").Int(int64(result.pid)).Str(", ").
		Int(int64(result.elapsed / time.Second)).Str("s elapsed)...").Colored(colors.Reset)
	a.note(tb.String())
}

// readEntry parses one registry file and reports whether the file was readable.
// A false result activates FAIL CLOSED. The caller counts the entry as occupied
// and does not assume that no job is running.
func (a *Admission) readEntry(path string) (entry, bool) {
	body, err := os.ReadFile(path) //nolint:gosec // a path this scan took from the registry directory itself
	if err != nil {
		return entry{}, false
	}

	pid, err := strconv.Atoi(field(body, "PID"))
	if err != nil || pid <= 0 {
		return entry{}, false
	}

	held := entry{
		path:  path,
		label: field(body, "LABEL"),
		pid:   pid,
		tree:  field(body, "TREE"),
		key:   field(body, "KEY"),
		log:   field(body, "LOG"),
		state: field(body, "STATE"),
	}
	if pgid, err := strconv.Atoi(field(body, "PGID")); err == nil {
		held.pgid = pgid
	}
	// A STARTED nobody wrote leaves the job as old as this instant, which
	// makes its elapsed time zero rather than fifty years. Elapsed time is
	// reported and never acted on, so a missing one costs a banner's accuracy
	// and nothing else.
	held.started = time.Now()
	if started, err := strconv.ParseInt(field(body, "STARTED"), 10, 64); err == nil {
		held.started = time.Unix(started, 0)
	}
	return held, true
}

// field answers the value of one FIELD=VALUE line, and the empty string when
// the field is absent. The first line that starts with the name wins, which is
// what the shell half's reader does.
func field(body []byte, name string) string {
	var tb textbuf.Buffer
	prefix := tb.Str(name).Byte('=').String()

	for line := range bytes.SplitSeq(body, []byte("\n")) {
		if bytes.HasPrefix(line, []byte(prefix)) {
			return string(line[len(prefix):])
		}
	}
	return ""
}

// entryName spells one registry file: the label, the process, and the suffix.
// The process is in the name so two jobs with the same label can coexist once
// more than one slot exists.
func entryName(label string, pid int, suffix string) string {
	var tb textbuf.Buffer
	return tb.Str(label).Byte('.').Int(int64(pid)).Str(suffix).String()
}

// swapSuffix answers the sibling of one registry file: its log, or its result.
func swapSuffix(path, suffix string) string {
	var tb textbuf.Buffer
	return tb.Str(strings.TrimSuffix(path, entrySuffix)).Str(suffix).String()
}

// reap drops an entry and its log together, which is what makes a follower
// hold its own descriptor rather than reopen a path.
func (a *Admission) reap(path string) {
	a.remove(path)
	a.remove(swapSuffix(path, logSuffix))
}

// remove deletes one registry file. An absent file is normal because another
// waiter can remove it first. A file that cannot be deleted requires a log
// line because an entry that cannot be removed holds a slot indefinitely.
func (a *Admission) remove(path string) {
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		a.note(errorLine(err))
	}
}

// jobsDir answers the absolute registry directory.
func (a *Admission) jobsDir() string {
	return a.abs(JobsDir)
}

// rel answers a registry path relative to the root, which is the form an entry
// records and the other half reads.
func (a *Admission) rel(path string) string {
	var tb textbuf.Buffer
	if trimmed, found := strings.CutPrefix(path, tb.Str(a.Root).Byte(filepath.Separator).String()); found {
		return trimmed
	}
	return path
}

// fileTime answers when a file was last written, or the zero time when it
// cannot be asked.
func fileTime(path string) time.Time {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}

// processGroup answers this process's group, or its pid when the group cannot
// be asked. The fallback is the safe direction: signalTarget refuses to signal
// its own group, and a group it cannot name is one it must not signal.
func processGroup() int {
	pgid, err := syscall.Getpgid(os.Getpid())
	if err != nil {
		return os.Getpid()
	}
	return pgid
}

// alive reports whether a process is still running a job.
//
// It queries the process table because the two less expensive checks answer
// different questions. `kill -0` tests whether this process can SIGNAL the
// target. Thus, another user's job appears absent even while it holds a slot.
// /proc gives the correct answer but is available only on Linux.
//
// A ZOMBIE counts as gone. It has exited, uses no CPU or memory, and cannot
// write more output. However, it remains listed until its parent reaps it. If
// a ZOMBIE counted as alive, a finished job would continue to hold its slot.
//
// A follower would also wait for a run that ended. Linux prints the state as
// one letter, and macOS appends flags to it. Therefore, the state is the first
// letter on both systems.
func alive(pid int) bool {
	if pid <= 0 {
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), aliveTimeout)
	defer cancel()

	var tb textbuf.Buffer
	//nolint:gosec // the argument is a number this package printed from a registry entry's PID field
	out, err := exec.CommandContext(ctx, "ps", "-o", "state=", "-p", tb.Int(int64(pid)).String()).Output()
	if err != nil {
		return false
	}
	reported := strings.TrimSpace(string(out))
	if reported == "" || strings.HasPrefix(reported, "Z") {
		return false
	}
	return true
}

// registryLock is the flock this package takes for the length of a scan.
type registryLock struct {
	file *os.File
}

// lockRegistry takes the registry lock, waiting up to lockWait for it.
//
// The lock covers the scan and its subsequent write. It covers nothing else.
// The lock is never held for the length of a job. The entries hold that claim,
// and a long-lived lock would prevent every waiter's banner.
func (a *Admission) lockRegistry() (registryLock, error) {
	path := filepath.Join(a.jobsDir(), lockName)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o644) //nolint:gosec // the registry lock is shared by every session on this machine
	if err != nil {
		return registryLock{}, err
	}

	deadline := time.Now().Add(lockWait)
	for {
		err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			return registryLock{file: file}, nil
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) || time.Now().After(deadline) {
			if closeErr := file.Close(); closeErr != nil {
				a.note(errorLine(closeErr))
			}
			return registryLock{}, err
		}
		time.Sleep(lockRetry)
	}
}

// release drops the registry lock. Closing the descriptor releases the flock,
// and the file itself stays: it is a lock rather than a record.
func (l registryLock) release(a *Admission) {
	if l.file == nil {
		return
	}
	if err := l.file.Close(); err != nil {
		a.note(errorLine(err))
	}
}
