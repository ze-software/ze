// Design: docs/architecture/fleet-config.md -- panic containment per managed client connection
// Related: managed_serve.go -- ManagedServer.handleConn, the managed client task's boundary
// Related: server.go -- NewServer and SetMetricsRegistry publish the plugin guards' counters
// Related: dispatch.go -- dispatchPluginRPC, the per-request engine-op task's boundary

package server

import (
	"sync"

	pluginipc "github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/taskguard"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// taskManagedClient is the task label of ManagedServer.handleConn in
// ze_task_panics_total{component="plugin"}.
const taskManagedClient = "managed-client"

// managedClientRestart tells the operator what a contained panic does to the
// managed client it hit.
const managedClientRestart = "that managed client's connection closes and its name is released; " +
	"the client connects again on its own schedule, and every other client keeps its connection"

// managedPanicTask is the panic boundary of one managed client connection.
type managedPanicTask struct {
	guard  *taskguard.Guard
	client taskguard.Task
}

// managedPanics builds the managed client guard once, on the first
// connection that defers it.
var managedPanics = sync.OnceValue(func() *managedPanicTask {
	guard := taskguard.New("plugin", managedLogger())
	return &managedPanicTask{guard: guard, client: guard.Task(taskManagedClient, managedClientRestart)}
})

// taskEngineOp is the task label of dispatchPluginRPC in
// ze_task_panics_total{component="plugin"}.
const taskEngineOp = "engine-op"

// engineOpRestart tells the operator what a contained panic does to the
// plugin request it hit.
const engineOpRestart = "that one plugin request is answered with an error and is not applied; " +
	"the plugin keeps its connection and its other requests are served"

// engineOpPanicTask is the panic boundary of one plugin-to-engine request.
type engineOpPanicTask struct {
	guard   *taskguard.Guard
	request taskguard.Task
}

// engineOpPanics builds the engine-op guard once, on the first request that
// defers it.
var engineOpPanics = sync.OnceValue(func() *engineOpPanicTask {
	guard := taskguard.New("plugin", logger())
	return &engineOpPanicTask{guard: guard, request: guard.Task(taskEngineOp, engineOpRestart)}
})

// engineOpPanicMessage is the error a request whose handler panicked is
// answered with. It names no panic value: the plugin is told that the engine,
// not its request, failed, and the operator finds the defect in the crash
// record.
func engineOpPanicMessage(method string, p *taskguard.Panic) string {
	var tb textbuf.Buffer
	tb.Str("internal Ze defect while handling ").Str(method).
		Str(": the request was not applied and may be sent again; the plugin connection stays open. ")
	if p.Report != "" {
		tb.Str("Crash record ").Str(p.Report).Str(": ")
	}
	tb.Str("run 'ze show crashes' for the details")
	return tb.String()
}

// setPluginPanicMetrics publishes ze_task_panics_total{component="plugin"} for
// every plugin connection task the engine runs: the connection readers, the
// connect-back handler, the internal plugin runner and the per-request engine
// op. The managed client task is published by NewManagedServer, which receives
// its own registry.
func setPluginPanicMetrics(reg metrics.Registry) {
	rpc.SetPanicMetrics(reg)
	pluginipc.SetPanicMetrics(reg)
	process.SetPanicMetrics(reg)
	engineOpPanics().guard.SetMetricsRegistry(reg)
}
