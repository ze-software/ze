// Design: docs/architecture/api/process-protocol.md -- panic containment at plugin connect-back
// Related: tls.go -- PluginAcceptor.handleConn, the connect-back task's boundary

package ipc

import (
	"sync"

	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/taskguard"
)

// taskConnectBack is the task label of PluginAcceptor.handleConn in
// ze_task_panics_total{component="plugin"}.
const taskConnectBack = "connect-back"

// connectBackRestart tells the operator what a contained connect-back panic
// does: the engine's wait for that plugin is the existing path that reports it.
const connectBackRestart = "the connecting plugin's connection closes before the engine receives it; " +
	"the engine's wait for that plugin times out and its start fails as for a plugin that never connected; " +
	"every other connect-back is served"

// acceptorPanics builds the connect-back guard once, on the first connection
// that defers it.
var acceptorPanics = sync.OnceValue(func() *connectBackPanicTask {
	guard := taskguard.New("plugin", slogutil.Logger("plugin.ipc"))
	return &connectBackPanicTask{guard: guard, connectBack: guard.Task(taskConnectBack, connectBackRestart)}
})

// connectBackPanicTask is the panic boundary of one connect-back handler.
type connectBackPanicTask struct {
	guard       *taskguard.Guard
	connectBack taskguard.Task
}

// SetPanicMetrics publishes the connect-back task's panic counter on reg. The
// engine's plugin server calls it when it receives its metrics registry.
func SetPanicMetrics(reg metrics.Registry) { acceptorPanics().guard.SetMetricsRegistry(reg) }
