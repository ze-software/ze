// Design: docs/guide/route-reflection.md -- role replay settlement.
// Related: rr.go -- both state transports admit replay before launching it.
package rr

import "context"

func (rr *routeReflector) replayParent() context.Context {
	if rr.downCtx == nil {
		return context.Background()
	}
	return rr.downCtx
}

// startPeerReplay admits this peer-UP lifecycle before launching its goroutine.
// The lifecycle MUST finish its admission and close done after all replay phases
// and its final readiness report, including on an optional-store failure.
func (rr *routeReflector) startPeerReplay(target peerTarget, initialReplay uint64) {
	rr.captureReplaySources(target)
	done := make(chan struct{})
	ctx, admitted := rr.replays.begin(rr.downCtx, done)
	if !admitted {
		return
	}
	go rr.replayForPeer(ctx, target, initialReplay, done)
}

// settleReplays joins every peer-up
// replay admitted before this call. Source validation/forward joins and
// per-route retention admission remain required for the GR cleanup protocol.
func (rr *routeReflector) settleReplays(ctx context.Context) error {
	if err := joinReplays(ctx, rr.replays.snapshot()); err != nil {
		return err
	}
	return rr.retryPeerReplays(ctx)
}
