// Design: docs/architecture/core-design.md — graceful restart peer-removal cleanup
// Overview: gr.go — GR plugin entry point and state-event dispatch
// Related: gr_state.go — grStateManager.removePeer clears timers + state
//
// Peer-removal handling. The GR plugin has no dedicated peer-removal callback:
// it learns a peer was deconfigured from a SessionStateDown carrying reason
// rpc.ReasonPeerRemoved, emitted unconditionally by the reactor's removePeer.
// This file turns that signal into terminal cleanup so a deconfigured peer does
// not retain routes and does not leak its per-peer ze_gr_* Prometheus series
// (the reactor cleans its own metrics; plugin metrics were previously orphaned).

package gr

// onPeerRemoved releases all GR/LLGR state for a peer removed from configuration
// and deletes its per-peer Prometheus series. A tombstone is recorded so a
// racing teardown "down" for the same peer cannot re-activate GR afterwards.
func (gp *grPlugin) onPeerRemoved(peerAddr string) {
	gp.state.removePeer(peerAddr) // stop restart/LLST timers, drop GR/LLGR state

	gp.mu.Lock()
	delete(gp.peerCaps, peerAddr)
	delete(gp.peerLLGRCaps, peerAddr)
	if gp.removedPeers == nil {
		gp.removedPeers = make(map[string]bool)
	}
	gp.removedPeers[peerAddr] = true
	gp.mu.Unlock()

	if m := grMetricsPtr.Load(); m != nil {
		m.staleRoutes.Delete(peerAddr)
		m.timerExpired.Delete(peerAddr)
	}
	logger().Debug("gr: peer removed, released GR state and per-peer metrics", "peer", peerAddr)
}

// consumeRemovedTombstone reports whether peerAddr was just removed and clears
// the tombstone. Used to suppress GR activation on a teardown "down" that races
// the removal signal (e.g. an Established peer emits both).
func (gp *grPlugin) consumeRemovedTombstone(peerAddr string) bool {
	gp.mu.Lock()
	defer gp.mu.Unlock()
	if gp.removedPeers[peerAddr] {
		delete(gp.removedPeers, peerAddr)
		return true
	}
	return false
}

// clearRemovedTombstone drops any removal tombstone for peerAddr. Called when the
// peer re-establishes so a genuine later restart still activates GR.
func (gp *grPlugin) clearRemovedTombstone(peerAddr string) {
	gp.mu.Lock()
	delete(gp.removedPeers, peerAddr)
	gp.mu.Unlock()
}
