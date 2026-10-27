// Design: docs/guide/config-editor.md -- Commit Confirmed: the daemon owns the window
// Related: confirm.go -- Window.do, the one place a request reaches the worker

package confirm

import (
	"errors"

	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/taskguard"
)

// taskWrite is the task label of one config write the worker runs (a commit,
// an accept, an abort, a write from outside an editor) in
// ze_task_panics_total{component="config-confirm"}.
const taskWrite = "write"

// writeRestart is what follows a contained write panic, in the words the
// guard's log line gives the operator.
const writeRestart = "only this config write ended, and its caller got an error; the " +
	"configuration may be partly applied, and the commit-confirm worker keeps serving " +
	"the next write and the open window's deadline"

// writeGuard is the panic boundary of every request the window's worker runs.
// The request comes from a gNMI Set, an SSH or web editor commit, an API config
// session or a managed push, whose own boundary sits on another goroutine and
// cannot see a panic here. Safe for concurrent use.
var writeGuard = taskguard.New("config-confirm", confirmLog)

// writeTask is the write task, declared once so its count exists before the
// first panic.
var writeTask = writeGuard.Task(taskWrite, writeRestart)

// SetPanicMetrics publishes ze_task_panics_total{component="config-confirm"} on
// reg. The daemon calls it once its metrics registry exists; counts recovered
// before are carried over.
func SetPanicMetrics(reg metrics.Registry) {
	writeGuard.SetMetricsRegistry(reg)
}

// write runs fn, one request, on the worker and answers its error. A Ze defect
// fn meets ends that request only: the caller gets an error that says so, and
// the worker goes on to the next request. The worker state fn may have changed
// before it panicked is left as it was, since the request is the unit; a
// confirmed commit whose apply panicked has its window armed (applyArmed), so
// what it applied still has a revert behind it.
func (w *Window) write(fn func() error) (err error) {
	defer writeTask.Contain(describeWrite, func(p *taskguard.Panic) { err = writeEndedByPanic(p) })
	return fn()
}

// describeWrite names the unit for the guard's log line and crash record. The
// worker serves one request at a time, so the request is the unit.
func describeWrite() taskguard.Context {
	return taskguard.Context{Unit: "config write on the commit-confirm worker"}
}

// writeEndedByPanic answers the caller in plain words: what ended, what state
// it may have left, and what to run next. The panic value stays in the log and
// the crash record, never in an answer a remote client reads.
func writeEndedByPanic(p *taskguard.Panic) error {
	next := "run 'ze show crashes' on the router and attach the newest record to a bug report"
	if p.Report != "" {
		next = "run 'ze show crashes name " + p.Report + "' on the router and attach it to a bug report"
	}
	return errors.New("an internal Ze defect ended this config write, which may be " +
		"partly applied: compare the running configuration with what you wrote before writing " +
		"again, and " + next)
}
