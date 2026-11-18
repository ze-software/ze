// Design: docs/guide/route-reflection.md -- role replay settlement.
// Related: server_handlers.go -- admission precedes the peer-up goroutine.
package rs

import "context"

// replayParent uses the process cancellation bound for mandatory replay. A
// directly constructed synchronous test instance has no process to cancel.
func (rs *routeServer) replayParent() context.Context {
	if rs.downCtx == nil {
		return context.Background()
	}
	return rs.downCtx
}

// settleReplays joins every peer-up
// replay already admitted, including superseded lifecycles, through their final
// engine call. This is replay settlement only: source worker joins and per-route
// retention admission are separate requirements of GR cleanup.
func (rs *routeServer) settleReplays(ctx context.Context) error {
	return joinReplays(ctx, rs.replays.snapshot())
}
