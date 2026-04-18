// Design: docs/architecture/api/process-protocol.md -- plugin signal handling
// Related: sdk.go -- Plugin.Run consumes the context returned here

package sdk

import (
	"context"
	"os/signal"
	"syscall"
)

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
// Internal (goroutine-mode) plugins share the process with ze, so ze's own
// main signal handler also catches SIGTERM -- the per-plugin handler is a
// belt-and-braces safety net. Subprocess (fork-mode) plugins rely on this
// helper as their only signal-cancellation path; without it they would be
// killed by the default Go signal disposition without running deferreds.
//
// Centralizing the signal set (SIGINT + SIGTERM) here means future
// additions (e.g. SIGHUP for live reload) update every plugin in one
// place.
func SignalContext() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
}
