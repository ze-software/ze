// Design: docs/architecture/api/process-protocol.md -- panic containment in the connection readers
// Related: conn.go -- Conn.readLoop, the frame reader's boundary
// Related: mux.go -- MuxConn.readLoop, the line router's boundary

package rpc

import (
	"sync"

	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/taskguard"
)

// The task names of the plugin connection readers, the task label of
// ze_task_panics_total{component="plugin"}.
const (
	taskRPCRead  = "rpc-read"
	taskRPCRoute = "rpc-route"
)

// readerRestart tells the operator what a contained reader panic does to the
// connection. Both readers end the same unit, so they share it.
const readerRestart = "the plugin connection closes: the engine handles the plugin as one whose process exited " +
	"and releases its commands, subscriptions and routes, and a plugin process sees its engine connection close; " +
	"every other plugin connection keeps running"

// readerPanicTasks is the panic boundary of the two goroutines that read a
// plugin connection: Conn.readLoop, which reads frames, and MuxConn.readLoop,
// which routes them.
type readerPanicTasks struct {
	guard *taskguard.Guard
	read  taskguard.Task
	route taskguard.Task
}

// readerPanics builds the guard once, on the first reader that defers it. It
// runs in the engine and in every plugin process built on this package; a
// plugin process has no metrics registry, so there it logs and records only.
var readerPanics = sync.OnceValue(func() *readerPanicTasks {
	guard := taskguard.New("plugin", slogutil.Logger("plugin.rpc"))
	return &readerPanicTasks{
		guard: guard,
		read:  guard.Task(taskRPCRead, readerRestart),
		route: guard.Task(taskRPCRoute, readerRestart),
	}
})

// SetPanicMetrics publishes the connection readers' panic counters on reg. The
// engine's plugin server calls it when it receives its metrics registry; a
// plugin process does not call it.
func SetPanicMetrics(reg metrics.Registry) { readerPanics().guard.SetMetricsRegistry(reg) }

// describeConn names the connection a contained reader panic ended: the label
// the owner set (the plugin name), or the generic name when none was set.
// label is written once after construction and read here without c.mu, as
// fireWatchdog reads it: a writer blocked under c.mu must not block the guard.
func (c *Conn) describeConn() taskguard.Context {
	if c.label == "" {
		return taskguard.Context{Unit: "plugin connection"}
	}
	return taskguard.Context{Unit: "plugin connection " + c.label}
}

// endAfterPanic is the frame reader's end: it records the panic as the
// reader's terminal error, which readFrame and Err then return, and closes the
// connection so the remote side sees it end. readLoop's deferred
// close(readerDone) runs after it, so a waiter reads the error.
func (c *Conn) endAfterPanic(p *taskguard.Panic) {
	var err error = p
	c.readerErr.Store(&err)
	if closeErr := c.Close(); closeErr != nil {
		slogutil.Logger("plugin.rpc").Debug("plugin connection close after a contained panic", "error", closeErr)
	}
}

// endAfterPanic is the line router's end: it records the panic as the
// connection's terminal error and closes the connection. readLoop's deferred
// calls then run as on any other reader exit: done closes, every pending
// answer ends with that error, and Requests closes, which is how the owner
// learns the connection is gone.
func (m *MuxConn) endAfterPanic(p *taskguard.Panic) {
	var err error = p
	m.readerErr.Store(&err)
	if closeErr := m.Close(); closeErr != nil {
		slogutil.Logger("plugin.rpc").Debug("plugin connection close after a contained panic", "error", closeErr)
	}
}
