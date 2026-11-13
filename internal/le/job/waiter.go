// Design: docs/contributing/running-commands.md -- waiting for another session's job
// Overview: job.go -- the queue that writes and withdraws this record
// Related: registry.go -- the entries a waiter record sits beside and is never read with
// Related: overview.go -- the report that lists the waiters
//
// A job that waits for a slot used to leave no trace: the registry showed who
// held the machine and never who was queued behind them. A waiting job now
// writes tmp/.ze-jobs/<label>.<pid>.<claim>.wait, and removes it on every way
// out of the wait.
//
// The record describes and decides nothing. No entry scan matches its suffix,
// so it never occupies a slot and admission answers what it answered before
// the record existed. A record whose process died stays on disk until
// `le scratch clean` prunes it (Prune, prune.go): no job removes it but its
// writer, and a reader tells it from a live one by the process table.

package job

import (
	"errors"
	"os"
	"strconv"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// waiterSuffix names a waiter record. It differs from every suffix an entry
// scan globs (entrySuffix), which is what keeps a waiter out of the slot count.
const waiterSuffix = ".wait"

// waitingField is when the wait started, in Unix seconds.
const waitingField = "WAITING"

// waiter is one waiter record, parsed.
type waiter struct {
	label string
	pid   int
	// since is when the job asked for a slot.
	since time.Time
	// who is the session and agent the record names, each empty when it names
	// none (ownerOf).
	who owner
	// command is the record's CMD line, read as data and never executed.
	command string
}

// waitNotice is this invocation's own waiter record, as the queue holds it.
// The zero value is "nothing written and nothing refused".
type waitNotice struct {
	// path is the record on disk, or "" while there is none.
	path string
	// refused says a write failed and was reported, so the queue does not try
	// again and does not repeat the report on every poll.
	refused bool
}

// announceWait writes this job's waiter record unless one is already on disk.
//
// A failure is reported once and never retried. It MUST NOT fail or delay the
// admission: the job keeps waiting exactly as it did before the record
// existed, and the report then does not list it. The caller MUST call
// withdrawWait with the same notice on every way out of its wait.
func (a *Admission) announceWait(job *pending, since time.Time, notice *waitNotice) {
	if notice.path != "" {
		return
	}
	if notice.refused {
		return
	}

	path, err := a.writeWaiter(job, since)
	if err != nil {
		notice.refused = true
		var tb textbuf.Buffer
		a.note(tb.Byte('[').Str(job.label).
			Str("] this wait is not recorded, so `le job report` will not list it: ").Err(err).String())
		return
	}
	notice.path = path
}

// withdrawWait removes this invocation's own waiter record, if it wrote one.
// It MUST be called with the notice announceWait filled, and is safe to call
// when nothing was written. The queue defers it, so every way out of the wait
// passes through here: admitted, attached, canceled, refused.
func (a *Admission) withdrawWait(notice *waitNotice) {
	if notice.path == "" {
		return
	}
	a.remove(notice.path)
	notice.path = ""
}

// writeWaiter creates one waiter record and answers its path.
//
// The name is unique the way an entry's is: the label, this process, and a
// claim the exclusive creation chose, so a reused PID never meets an earlier
// record. It needs no registry lock, because no scan that decides admission
// reads it.
func (a *Admission) writeWaiter(job *pending, since time.Time) (string, error) {
	pid := os.Getpid()

	file, err := os.CreateTemp(a.jobsDir(), entryName(job.label, pid, ".*"+waiterSuffix))
	if err != nil {
		return "", err
	}

	// As in an entry (take), every text value goes through writeField: the
	// record MUST NOT carry a caller-controlled value containing a line break,
	// because readWaiter takes the owner from the first SESSION and AGENT
	// lines, and the command is written before them.
	var tb textbuf.Buffer
	writeField(&tb, "LABEL", job.label)
	tb.Str("PID=").Int(int64(pid)).Byte('\n')
	tb.Str(waitingField).Byte('=').Int(since.Unix()).Byte('\n')
	writeField(&tb, commandField, commandLine(job.argv))
	writeOwner(&tb, job.who)

	_, writeErr := file.WriteString(tb.String())
	closeErr := file.Close()
	// Every session's report reads the record, as every session's scan reads
	// an entry.
	chmodErr := os.Chmod(file.Name(), 0o644) //nolint:gosec // the report of every session on this machine reads a waiter record
	if err := errors.Join(writeErr, closeErr, chmodErr); err != nil {
		a.remove(file.Name())
		return "", err
	}
	return file.Name(), nil
}

// readWaiter parses one waiter record and reports whether it names a waiter.
//
// A record that cannot be read, or that names no process, no label or no
// start, is not a waiter anybody can show. It holds no slot either way, so it
// is left out and left alone.
//
// Each field is its first line in the record (field). The writer keeps every
// value on one line (writeField), so no value supplies a line read here.
func readWaiter(path string) (waiter, bool) {
	body, err := os.ReadFile(path) //nolint:gosec // a path the report took from the registry directory itself
	if err != nil {
		return waiter{}, false
	}

	pid, err := strconv.Atoi(field(body, "PID"))
	if err != nil {
		return waiter{}, false
	}
	if pid <= 0 {
		return waiter{}, false
	}
	label := field(body, "LABEL")
	if !validLabel(label) {
		return waiter{}, false
	}
	since, err := strconv.ParseInt(field(body, waitingField), 10, 64)
	if err != nil {
		return waiter{}, false
	}

	return waiter{
		label:   label,
		pid:     pid,
		since:   time.Unix(since, 0),
		who:     ownerOf(body),
		command: field(body, commandField),
	}, true
}
