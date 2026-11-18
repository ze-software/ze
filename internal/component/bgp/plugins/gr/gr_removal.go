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
// and deletes its per-peer Prometheus series. Its immutable owner cannot
// borrow a same-address successor's capability; no tombstone history is kept.
func (gp *grPlugin) onPeerRemoved(source grPeer) {
	gp.state.removePeer(source) // stop restart/LLST timers, drop GR/LLGR state

	gp.mu.Lock()
	delete(gp.peerCaps, source.owner)
	delete(gp.peerLLGRCaps, source.owner)
	delete(gp.sentLLGRFamilies, source.owner)
	gp.mu.Unlock()

	if m := grMetricsPtr.Load(); m != nil {
		m.staleRoutes.Delete(source.peer())
		m.timerExpired.Delete(source.peer())
	}
	logger().Debug("gr: peer removed, released GR state and per-peer metrics", "peer", source)
}
