// Design: docs/architecture/api/process-protocol.md -- panic containment in the internal plugin runner
// Related: process.go -- startInternal, the runner goroutine's boundary

package process

import (
	"sync"

	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/taskguard"
)

// taskRun is the task label of the internal plugin runner in
// ze_task_panics_total{component="plugin"}.
const taskRun = "run"

// runRestart tells the operator what a contained runner panic does: the plugin
// ends, and the engine takes the path it takes for an internal plugin that
// returned on its own.
const runRestart = "the internal plugin ends: its connection closes and the engine releases its commands, " +
	"subscriptions and routes, as for a plugin that exited; every other plugin keeps running"

// runnerPanicTask is the panic boundary of the goroutine that runs an internal
// plugin.
type runnerPanicTask struct {
	guard *taskguard.Guard
	run   taskguard.Task
}

// runnerPanics builds the runner guard once, on the first internal plugin
// that defers it.
var runnerPanics = sync.OnceValue(func() *runnerPanicTask {
	guard := taskguard.New("plugin", slogutil.Logger("plugin"))
	return &runnerPanicTask{guard: guard, run: guard.Task(taskRun, runRestart)}
})

// SetPanicMetrics publishes the runner task's panic counter on reg. The
// engine's plugin server calls it when it receives its metrics registry.
func SetPanicMetrics(reg metrics.Registry) { runnerPanics().guard.SetMetricsRegistry(reg) }
