// Design: docs/architecture/testing/runner-architecture.md -- the step trace the report prints
// Overview: runner_exec.go -- the command loop that records these steps
// Related: runner_output_assert.go -- recordStep, the assertion half of the same trace

package runner

import (
	"io"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/test/trace"
)

// stepKindExec is the trace.StepResult kind for a command the runner RAN, as
// opposed to stepKindExpect, which is an assertion about what it produced.
const stepKindExec = "exec"

// recordExecStep records the command the runner is about to run: the argv it
// built, and where the named stdin= block went. It is written before the
// process starts, so on its own it does not say the process started:
// execClock.stamp adds that, and returns the step's index for it.
//
// The .ci says what to run and the runner can rewrite it. A `ze -` daemon
// launch becomes `ze [flags] start <file>`, and a ze-peer line carrying no `-`
// gains a path argument. Both rewrites are deliberate, and neither appeared
// anywhere an author reads, so a test whose stimulus was replaced looked
// exactly like a test whose stimulus was honored: test/ui/bgp-decode-pcap-stdin.ci
// asserted a decoded capture while the capture reached the child as a file path
// it never wrote, and passed.
//
// The step always passes, because it is an observation rather than an
// assertion. It carries everything in Assert: the human trace prints Detail
// only for a failed step, and this step is what the reader of a PASSING run
// came for (trace.writeHuman).
func (r *Record) recordExecStep(binPath string, args []string, blockName string, piped bool) int {
	var b textbuf.Buffer
	b.Str(binPath)
	for _, arg := range args {
		b.Byte(' ').Str(arg)
	}

	b.Str("  [")
	switch {
	case blockName == "":
		b.Str("no stdin block")
	case piped:
		b.Str("stdin=").Str(blockName).Str(" piped")
	default:
		b.Str("stdin=").Str(blockName).Str(" written to a file, named in the argv above")
	}
	b.Byte(']')

	r.StepTrace = append(r.StepTrace, trace.StepResult{
		Step: len(r.StepTrace) + 1, Kind: stepKindExec, Assert: b.String(), Passed: true,
	})
	return len(r.StepTrace) - 1
}

// execClock places every command of one .ci run on that run's deadline clock.
//
// A timeout says only that the budget ran out, not where it went. The base
// bgp-delay-open TIME30 printed its daemon's first timestamped line 21.7s into
// a 30s budget, where a pass prints it under 3s, and nothing could say whether
// the runner was late to exec, the exec was slow to return, or the daemon was
// slow to start. Each exec step therefore gains three marks: the clock's start
// in wall time (the daemon stamps its own log lines in wall time), when Start
// returned, and when the process first wrote to stderr.
//
// Owned by one runOrchestrated call: created beside its deadline and stamped
// by its deferred stamp, so it is not shared between goroutines. firstOutput,
// which the process's output copier writes, is safe for concurrent use.
type execClock struct {
	start time.Time
	marks []execMark
}

// execMark is one command that started: its exec step, when Start returned,
// and the watch on its stderr.
type execMark struct {
	step    int
	started time.Duration
	pid     int
	stderr  *firstOutput
}

func newExecClock(start time.Time) *execClock {
	return &execClock{start: start}
}

// watch wraps a command's stderr so the clock sees its first write.
func (c *execClock) watch(w io.Writer) *firstOutput {
	return &firstOutput{w: w, start: c.start}
}

// mark records a command whose Start returned without error.
func (c *execClock) mark(step, pid int, stderr *firstOutput) {
	c.marks = append(c.marks, execMark{step: step, started: time.Since(c.start), pid: pid, stderr: stderr})
}

// execClockLayout matches the daemon's slog timestamps, so the reader can
// subtract one from the other.
const execClockLayout = "2006-01-02T15:04:05.000Z07:00"

// stamp appends the marks to each started command's exec step. It runs when
// the run returns, so the first-output mark covers the whole run. A command
// whose Start failed carries no mark: the run's error says why.
func (c *execClock) stamp(rec *Record) {
	for _, m := range c.marks {
		var b textbuf.Buffer
		b.Str("  [clock start ").Str(c.start.UTC().Format(execClockLayout))
		b.Str(", started +").Float(m.started.Seconds(), 3).Str("s pid ").Int(int64(m.pid))
		at, seen := m.stderr.at()
		if seen {
			b.Str(", first stderr +").Float(at.Seconds(), 3).Str("s]")
		} else {
			b.Str(", no stderr before the run ended]")
		}
		rec.StepTrace[m.step].Assert += b.String()
	}
}

// firstOutput passes writes through and keeps the clock offset of the first
// one. Safe for concurrent use.
type firstOutput struct {
	w     io.Writer
	start time.Time
	// offset is the first write's offset from start plus one nanosecond, so
	// zero is the guard for "nothing written yet" and a write at offset zero
	// is still seen.
	offset atomic.Int64
}

func (f *firstOutput) Write(p []byte) (int, error) {
	f.offset.CompareAndSwap(0, int64(time.Since(f.start))+1)
	return f.w.Write(p)
}

// at answers the first write's offset, and whether any write happened.
func (f *firstOutput) at() (time.Duration, bool) {
	v := f.offset.Load()
	if v == 0 {
		return 0, false
	}
	return time.Duration(v - 1), true
}
