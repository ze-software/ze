// Design: docs/architecture/api/process-protocol.md -- plugin signal handling
// Related: sdk.go -- Plugin.Run consumes the context returned here

package sdk

import (
	"context"
	"os/signal"
	"sync/atomic"
	"syscall"
)

// hostOwnsSignals is set once by a daemon that runs plugins in its own process
// and stops them itself (HostOwnsSignals). It is process-wide on purpose: a
// signal handler is process-wide too.
var hostOwnsSignals atomic.Bool

// HostOwnsSignals declares that this process is a daemon hosting in-process
// plugins, and that its shutdown sequence, not a signal, ends them. After it,
// SignalContext registers no handler.
//
// The daemon MUST call it before the first plugin starts. An in-process plugin
// that registered SIGTERM received it in the same instant as the daemon, and
// canceled while the daemon was still granting a running config reload its
// shutdown grace: the reload then waited on plugins that were already gone,
// and a routing plugin's exit tore the sessions down ahead of the ordered stop.
// The daemon's stop closes each in-process plugin's connection
// (process.Process.Stop), which is what unblocks its Run.
func HostOwnsSignals() { hostOwnsSignals.Store(true) }

// SignalContext returns a context that cancels when the plugin process
// receives SIGINT or SIGTERM, plus a CancelFunc that releases the handler
// when Run returns. Every plugin's runEngine/main entry point should use
// this at the top of its lifecycle so in-flight blocking calls (backend
// Apply, long-running IPC waits) unblock cleanly on daemon shutdown.
//
// Typical use:
//
//	func runEngine(conn net.Conn) int {
//	    p := sdk.NewWithConn("myplugin", conn)
//	    defer p.Close()
//
//	    ctx, cancel := sdk.SignalContext()
//	    defer cancel()
//
//	    // ... register callbacks ...
//
//	    if err := p.Run(ctx, sdk.Registration{...}); err != nil {
//	        return 1
//	    }
//	    return 0
//	}
//
// Internal (goroutine-mode) plugins share the process with the ze daemon,
// which calls HostOwnsSignals: the returned context then cancels only through
// the CancelFunc, and the daemon ends the plugin by closing its connection.
// Subprocess (fork-mode) plugins rely on this helper as their only
// signal-cancellation path; without it they would be killed by the default Go
// signal disposition without running deferreds.
//
// Centralizing the signal set (SIGINT + SIGTERM) here means future
// additions (e.g. SIGHUP for live reload) update every plugin in one
// place.
func SignalContext() (context.Context, context.CancelFunc) {
	if hostOwnsSignals.Load() {
		return context.WithCancel(context.Background())
	}
	return signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
}
