// Design: docs/architecture/core-design.md -- the registry that IS the claim
// Detail: treehash.go -- the tree an entry records
// Detail: jobkey.go -- the work an entry records
//
// tmp/.ze-jobs/<label>.<pid>.<claim>.job records one slot claim. Only its
// holder removes its files. A waiter counts live or unassessable work as
// occupied and ignores demonstrably dead work without changing its evidence.
//
// Readers use the fields, not the filename, for holder identity. The unique
// claim suffix keeps retained evidence safe across PID reuse and repeat runs.
// The tree hash lets a second asker share the result of equivalent work.

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

// One job owns three suffixes in the registry: its entry, progress log and
// completed verdict. Waiters never remove any of them.
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
	label string
	argv  []string
	// tree fingerprints the inputs this job's LABEL reads (InputHash,
	// treehash.go), which is what decides whether a running job's verdict can
	// serve this one. It is the whole checkout for a label that declares no
	// inputs of its own.
	tree      string
	key       string
	mayAttach bool
	// treeStale indicates that this job's fingerprint predates its admission
	// because the job waited. The inputs can change while a job waits.
	// Therefore, the fingerprint is calculated again at admission, but only for
	// a job that waited.
	treeStale bool
}

// outcome is what one scan decided.
type outcome struct {
	state state
	// err refuses a scan whose inputs could not be measured.
	err error
	// entry is the registry entry, relative to the root: this job's when the
	// state is claimed, the holder's when it is attach or busy.
	entry string
	// log is that entry's log, relative to the root.
	log string
	// pid is the holder's process, when the state is attach or busy.
	pid int
	// holder names what is occupying a slot, for the waiting banner.
	holder  string
	elapsed time.Duration
	// liveness is a process-query failure, not evidence of a free slot.
	liveness error
	started  time.Time
}

// entry is one registry file, parsed.
type entry struct {
	// path is the absolute path of the entry file.
	path  string
	label string
	pid   int
	tree  string
	key   string
	// log is the entry's LOG field as written: relative to the root.
	log     string
	state   string
	started time.Time
}

// claim asks the registry once, under the registry lock, and answers what it
// decided.
func (a *Admission) claim(ctx context.Context, job *pending) outcome {
	lock, err := a.lockRegistry(ctx)
	if err != nil {
		// The process failed to inspect the registry, so it does not admit a job.
		return outcome{state: stateBusy, holder: "registry lock"}
	}
	defer lock.release(a)

	return a.scanAndClaim(ctx, job)
}

// scanAndClaim counts live and unassessable entries, then shares an equivalent
// job, claims a slot, or reports the holder. Foreign files are never changed.
//
// MUST run with the registry lock. It counts directory entries and then writes
// an entry. If two scans overlap, both detect the same free slot.
func (a *Admission) scanAndClaim(ctx context.Context, job *pending) outcome {
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
		if err := ctx.Err(); err != nil {
			return outcome{err: err}
		}
		held, ok := a.readEntry(path)
		if !ok {
			// Age cannot prove that an unreadable entry's process has ended.
			// Keep its capacity occupied and leave all of its files intact.
			occupied++
			if result.holder == "?" {
				var tb textbuf.Buffer
				result.holder = tb.Str("unreadable entry ").Str(filepath.Base(path)).String()
				result.entry = a.rel(path)
			}
			continue
		}

		running, queryErr := alive(ctx, held.pid)
		if err := ctx.Err(); err != nil {
			return outcome{err: err}
		}
		if queryErr == nil {
			if !running {
				continue
			}
		}

		if share == nil && queryErr == nil {
			shared, err := a.shares(job, held)
			if err != nil {
				return outcome{err: err}
			}
			if shared {
				share = &held
			}
		}

		occupied++
		if result.holder == "?" {
			result.holder = held.label
			result.pid = held.pid
			result.elapsed = now.Sub(held.started)
			result.entry = a.rel(path)
			result.liveness = queryErr
			// The log the waiting banner reads the holder's current stage from.
			result.log = held.log
		}
	}

	if share != nil {
		return outcome{state: stateAttach, entry: a.rel(share.path), log: share.log, pid: share.pid}
	}
	if occupied >= a.Slots {
		return result
	}
	if err := ctx.Err(); err != nil {
		return outcome{err: err}
	}
	return a.take(job, now)
}

// shares reports whether a running job performs the asker's work over the
// asker's inputs. Both conditions must be true before the asker uses its
// verdict.
//
// The inputs are the ones the LABEL reads (InputHash, treehash.go), never the
// whole checkout. Nine sessions write this checkout, so a journal row somebody
// else added voided every match while the comparison was whole-tree, and two
// identical lints ran from scratch rather than one serving both.
//
// A value that cannot be measured matches no value. It also does not match an
// unmeasured value from another job. An unmeasured fingerprint is not a
// matching fingerprint, and an unmeasured key is not a matching key.
//
// The asker's inputs are measured again here when it waited, because the
// holder wrote a fingerprint taken at its own claim and the asker still
// carries one taken before its wait. Something changes during almost every
// wait, so comparing those two answers "different inputs" for a job that is
// doing identical work, and sharing would never happen after a wait. take
// measures again too, and only for the job it admits, which is too late to
// decide this.
//
// The measurement costs three git calls, so the key is compared FIRST. A job
// waiting behind unrelated work cannot share whatever its inputs say, and it
// pays nothing.
func (a *Admission) shares(job *pending, held entry) (bool, error) {
	if !job.mayAttach || held.state != "running" || held.label != job.label {
		return false, nil
	}
	if job.key == "" || job.key == Unknown || held.key != job.key {
		return false, nil
	}
	if job.treeStale {
		tree, err := InputHash(a.Root, job.label)
		if err != nil {
			return false, err
		}
		job.tree = tree
		job.treeStale = false
	}
	if job.tree == "" || job.tree == Unknown || held.tree != job.tree {
		return false, nil
	}
	return true, nil
}

// take creates a unique claim and its log without overwriting earlier evidence.
// It MUST run under the registry lock, which keeps another scan from reading
// the entry before its fields have been written.
func (a *Admission) take(job *pending, now time.Time) (result outcome) {
	// A later asker uses TREE to attach, so TREE must identify the inputs that
	// this job will judge. A job can wait behind a twenty minute holder. During
	// that wait, the inputs the job originally requested can change.
	if job.treeStale {
		tree, err := InputHash(a.Root, job.label)
		if err != nil {
			return outcome{err: err}
		}
		job.tree = tree
	}

	pid := os.Getpid()
	file, err := os.CreateTemp(a.jobsDir(), entryName(job.label, pid, ".*"+entrySuffix))
	if err != nil {
		return outcome{err: err}
	}
	if err := file.Close(); err != nil {
		a.remove(file.Name())
		return outcome{err: err}
	}
	defer func() {
		if result.state != stateClaimed {
			a.remove(file.Name())
		}
	}()
	rel := a.rel(file.Name())
	logRel := swapSuffix(rel, logSuffix)
	logFile, err := os.OpenFile(a.abs(logRel), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644) //nolint:gosec // the owned registry log is shared with followers
	if err != nil {
		return outcome{err: err}
	}
	defer func() {
		if result.state != stateClaimed {
			a.remove(a.abs(logRel))
		}
	}()
	if err := logFile.Close(); err != nil {
		return outcome{err: err}
	}
	// A completed receipt can outlive its entry. Never select that name for
	// new work, even if the temporary filename generator reused its suffix.
	if _, err := os.Lstat(a.abs(swapSuffix(rel, resultSuffix))); !errors.Is(err, fs.ErrNotExist) {
		if err != nil {
			return outcome{err: err}
		}
		return outcome{err: errors.New("the new job's result path already exists; retry admission without changing that file")}
	}

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

	if err := os.WriteFile(file.Name(), written, 0o644); err != nil { //nolint:gosec // this invocation exclusively created the entry above
		return outcome{err: err}
	}
	if err := os.Chmod(file.Name(), 0o644); err != nil { //nolint:gosec // followers read the completed registry entry
		return outcome{err: err}
	}

	return outcome{state: stateClaimed, entry: rel, log: logRel, pid: pid, started: now}
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

	file, err := os.OpenFile(a.abs(swapSuffix(entryRel, resultSuffix)), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644) //nolint:gosec // followers read the owned result; an existing file is never overwritten
	if err != nil {
		a.note(errorLine(err))
		return
	}
	if _, err := file.WriteString(body); err != nil {
		a.note(errorLine(err))
	}
	if err := file.Close(); err != nil {
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
//
// The banner reports observed log silence, never a verdict that silence proves
// a stalled process. Missing metadata stays unknown, not a zero-age holder.
func (a *Admission) reportBusy(label string, result outcome) {
	colors := textbuf.C
	var tb textbuf.Buffer
	tb.SetColor(a.Color)
	tb.Colored(colors.BrightYellow).Byte('[').Str(label).Str("] waiting: ").Str(result.holder)
	switch {
	case result.pid == 0:
		tb.Str(" (capacity occupied, holder identity unavailable)")
	case result.liveness != nil:
		tb.Str(" (pid ").Int(int64(result.pid)).Str(", liveness unknown: ").Err(result.liveness).Byte(')')
	default:
		tb.Str(" running (pid ").Int(int64(result.pid)).Str(", ").
			Int(int64(result.elapsed / time.Second)).Str("s elapsed)")
	}
	if result.entry != "" {
		tb.Str("; entry ").Str(result.entry)
	}
	if result.log != "" {
		written := fileTime(a.abs(result.log))
		if written.IsZero() {
			tb.Str("; log progress unavailable: ").Str(result.log)
		} else {
			silence := max(time.Since(written), 0)
			tb.Str("; ").Str(result.log).Str(" has not grown for ").
				Int(int64(silence / time.Second)).Byte('s')
			if silence > a.Stall {
				tb.Str(" (quiet window ").Int(int64(a.Stall / time.Second)).
					Str("s exceeded; holder kept occupied, ask its owner)")
			}
		}
		if stage := lastLine(a.abs(result.log)); stage != "" {
			tb.Str(": ").Str(stage)
		}
	}

	tb.Str("...").Colored(colors.Reset)
	a.note(tb.String())
}

// The bounds on the progress line a waiting job reports.
const (
	// progressTailMax is how much of a holder's log is read to find its last
	// line. A stage banner is one short line, and a log is unbounded: reading
	// the tail alone keeps the cost of a banner independent of how long the
	// holder has been running.
	progressTailMax = 4096
	// progressLineMax is how much of that line is printed. One banner every
	// thirty seconds goes into a reader's transcript, so a stage that prints a
	// whole compiler command does not take a terminal width with it.
	progressLineMax = 100
)

// lastLine answers the final non-empty line of a file, bounded, or "" when
// there is no such line or the file cannot be read.
//
// A file this cannot read is not an error to report: the caller is a banner,
// and a holder writing its log from another session can rotate or remove it
// between the scan and this read.
func lastLine(path string) string {
	file, err := os.Open(path) //nolint:gosec // a log path this scan took from the registry entry it just read
	if err != nil {
		return ""
	}
	defer file.Close() //nolint:errcheck // the log is only read

	info, err := file.Stat()
	if err != nil {
		return ""
	}
	size := info.Size()
	from := int64(0)
	if size > progressTailMax {
		from = size - progressTailMax
	}

	tail := make([]byte, size-from)
	read, err := file.ReadAt(tail, from)
	if err != nil && read == 0 {
		return ""
	}

	return lastLineOf(tail[:read])
}

// lastLineOf answers the final non-empty line of one block of log bytes.
//
// The block is a TAIL, so its first line can be a fragment of a longer one.
// That fragment is still the answer when it is the only line present: a holder
// whose stage banner is longer than the tail is better described by its end
// than by nothing at all.
func lastLineOf(tail []byte) string {
	for end := len(tail); end > 0; {
		start := bytes.LastIndexByte(tail[:end], '\n') + 1
		line := strings.TrimRight(string(tail[start:end]), " \t\r")
		if line != "" {
			return truncate(line, progressLineMax)
		}
		end = start - 1
	}
	return ""
}

// truncate shortens a line to at most limit bytes and says that it did.
//
// The cut lands on a rune boundary. A banner reaches a terminal and a
// transcript, and half a rune is a broken character in both, so the range loop
// over the string is what decides where the cut can be.
func truncate(line string, limit int) string {
	if len(line) <= limit {
		return line
	}

	cut := 0
	for index := range line {
		if index > limit {
			break
		}
		cut = index
	}

	var tb textbuf.Buffer
	return tb.Str(line[:cut]).Str("...").String()
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
		log:   registryLog(field(body, "LOG")),
		state: field(body, "STATE"),
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

// registryLog answers a LOG field that names a file inside the registry, and
// "" for one that names anything else.
//
// The field is written by ANOTHER session's process, and three consumers open
// or stat what it names: the liveness test, a follower replaying a shared run,
// and the waiting banner reading the holder's current stage. A relative path
// with .. in it, or an absolute one, would make each of them read a file
// outside the registry and put its last line into this session's transcript.
// The guard is here rather than at those three call sites so that one check
// covers every reader of the field, including the next one.
//
// A refused path reads as "this entry named no log", which is a state the
// package already handles everywhere: the holder keeps its slot, nothing is
// replayed, and the banner carries no stage.
func registryLog(named string) string {
	if named == "" || filepath.IsAbs(named) {
		return ""
	}
	clean := filepath.Clean(filepath.FromSlash(named))
	if !strings.HasPrefix(clean, filepath.FromSlash(JobsDir)+string(filepath.Separator)) {
		return ""
	}
	return filepath.ToSlash(clean)
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

// remove deletes only a file this invocation created. Callers MUST hold that
// ownership, not infer it from the file's age, PID or session identity.
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

// processGroup records this process's group as metadata. If the group cannot
// be queried, the process PID is recorded instead. Waiters never signal it.
func processGroup() int {
	pgid, err := syscall.Getpgid(os.Getpid())
	if err != nil {
		return os.Getpid()
	}
	return pgid
}

// alive reports whether a process is running, or why its state is unknown.
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
func alive(parent context.Context, pid int) (bool, error) {
	if pid <= 0 {
		return false, errors.New("a process query needs a positive PID")
	}

	ctx, cancel := context.WithTimeout(parent, aliveTimeout)
	defer cancel()

	var tb textbuf.Buffer
	//nolint:gosec // the argument is a number this package printed from a registry entry's PID field
	out, err := exec.CommandContext(ctx, "ps", "-o", "state=", "-p", tb.Int(int64(pid)).String()).Output()
	reported := strings.TrimSpace(string(out))
	if err != nil {
		// ps exits 1 with no output when the selected PID is absent. Every
		// other failure, including cancellation, leaves liveness unknown.
		var exit *exec.ExitError
		if ctx.Err() == nil && errors.As(err, &exit) {
			if exit.ExitCode() == 1 && reported == "" && len(exit.Stderr) == 0 {
				return false, nil
			}
		}
		return false, err
	}
	if reported == "" {
		return false, errors.New("the process query returned no state")
	}
	return !strings.HasPrefix(reported, "Z"), nil
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
func (a *Admission) lockRegistry(ctx context.Context) (registryLock, error) {
	path := filepath.Join(a.jobsDir(), lockName)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o644) //nolint:gosec // the registry lock is shared by every session on this machine
	if err != nil {
		return registryLock{}, err
	}

	deadline := time.Now().Add(lockWait)
	for {
		if err := ctx.Err(); err != nil {
			if closeErr := file.Close(); closeErr != nil {
				a.note(errorLine(closeErr))
			}
			return registryLock{}, err
		}
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
		pause(ctx, lockRetry)
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
