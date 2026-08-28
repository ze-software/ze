// Design: docs/architecture/core-design.md -- sharing a running job instead of running it twice
// Detail: registry.go -- the entry a follower watches, and the result it reads
//
// Serialization alone causes eight sessions to queue for eight runs of the
// same work. In a shared checkout, most sessions request the SAME job on the
// SAME tree. A second session follows a running job when the label, tree, and
// work key match. It copies that job's log to its own stdout instead of queuing
// a duplicate. It then exits with that job's code. One run serves both.

package job

import (
	"io"
	"os"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// attach follows a running job to completion and returns its recorded verdict.
//
// The second return value reports whether anything was OBSERVED. If a holder
// leaves no result, this value is false. The caller then returns to the queue
// and runs the job. Repeating the work is preferable to accepting a verdict
// that no process observed.
func (a *Admission) attach(label string, target outcome) (int, bool) {
	colors := textbuf.C
	var tb textbuf.Buffer
	tb.SetColor(a.Color)
	a.note(tb.Colored(colors.BrightCyan).Byte('[').Str(label).
		Str("] attaching to the ").Str(label).Str(" already running for this tree (pid ").
		Int(int64(target.pid)).Str("): one run answers both").Colored(colors.Reset).String())

	a.follow(target)

	// The holder writes its result before it drops its entry, so this covers
	// only the moment between its last log line and its exit.
	deadline := time.Now().Add(a.ResultWait)
	for {
		if code, ok := a.readResult(target.entry); ok {
			tb.Reset()
			tb.SetColor(a.Color)
			a.note(tb.Colored(colors.BrightCyan).Byte('[').Str(label).Str("] the shared ").Str(label).
				Str(" finished with exit ").Int(int64(code)).Colored(colors.Reset).String())
			return code, true
		}
		if time.Now().After(deadline) {
			break
		}
		time.Sleep(time.Second)
	}

	tb.Reset()
	tb.SetColor(a.Color)
	a.note(tb.Colored(colors.BrightYellow).Byte('[').Str(label).Str("] the job we followed (pid ").
		Int(int64(target.pid)).Str(") ended without recording a result; nothing was observed, so back to the queue").
		Colored(colors.Reset).String())
	return 0, false
}

// follow copies a running job's complete log to this job's stdout and returns
// when that job ends.
//
// follow uses a descriptor that THIS job keeps open. It never reopens a path.
// The holder records its result and then removes its entry and log together.
// A follower that reopened the path after removal would find nothing and lose
// the end of the run. The open descriptor retains the file until follow closes
// it. Thus, the copy after entry removal receives the complete tail instead of
// racing against the holder's exit.
//
// If follow cannot open a log, this job waits for the holder and does not replay
// the log. A job whose entry named no log has the same behavior.
func (a *Admission) follow(target outcome) {
	var file *os.File
	if target.log != "" {
		opened, err := os.Open(a.abs(target.log)) //nolint:gosec // the path came from a registry entry this scan just read
		if err == nil {
			file = opened
		}
	}

	for {
		a.copyTail(file)

		// The entry going is the job ending: it is removed after the result is
		// recorded. A killed job leaves its entry behind, so the process is
		// asked too.
		if !exists(a.abs(target.entry)) || !alive(target.pid) {
			a.copyTail(file)
			break
		}
		time.Sleep(a.Poll)
	}

	if file != nil {
		if err := file.Close(); err != nil {
			a.note(errorLine(err))
		}
	}
}

// copyTail moves whatever has arrived in the log since the last read to this
// job's stdout. The descriptor keeps its position, so the next round continues
// where this one stopped and no byte is printed twice.
func (a *Admission) copyTail(file *os.File) {
	if file == nil || a.Out == nil {
		return
	}
	if _, err := io.Copy(a.Out, file); err != nil {
		a.note(errorLine(err))
	}
}

// exists reports whether a path is still there, which is how a follower reads
// "the job I am following has finished".
func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
