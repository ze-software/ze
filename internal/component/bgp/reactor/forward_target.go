// Design: docs/architecture/api/process-protocol.md -- exact fast-path handback targets.
package reactor

import "github.com/ze-software/ze/pkg/plugin/rpc"

// forwardTarget captures the UP token from this snapshot's original physical
// Session. Encoding facts can precede UP; caching its provisional token would
// make a later handback stale. A reconnect never substitutes its new Session.
func (facts *peerForwardFacts) forwardTarget() rpc.ForwardTarget {
	if facts.dissemination == nil {
		return rpc.ForwardTarget{}
	}
	return capturedForwardTarget(facts.dissemination.peer, facts.dissemination.session)
}

// capturedForwardTarget MUST use the Session from the producing facts or item,
// never currentSession at delivery. The zero fields preserve missing capture as
// invalid input; a reader cannot invent a destination owner or session.
// Its caller MUST NOT hold peer.mu. The read lock pairs with replay receipt
// issuance so a received UPDATE cannot capture the old token between mint/store.
func capturedForwardTarget(peer *Peer, session *Session) rpc.ForwardTarget {
	if peer == nil {
		return rpc.ForwardTarget{}
	}
	peer.mu.RLock()
	defer peer.mu.RUnlock()
	target := rpc.ForwardTarget{Destination: peer.settings.PeerKey().String(), DestinationOwner: peer.sourceOwner}
	if session != nil {
		target.DestinationSession = session.initialReplay.Load()
	}
	return target
}
