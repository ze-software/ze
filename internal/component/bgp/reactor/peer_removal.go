// Design: docs/architecture/core-design.md — peer add/remove/lookup
// Related: reactor_peers.go — removePeer and doRemovePeer, the API removal
// Related: reactor_dynamic.go — removeDynamicPeer, a dynamic member's removal
// Related: reactor_api.go — the reload's removal and the undo of its add

package reactor

import (
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// peerRemovalEvents is where a removal tells the plugins that a peer is gone.
// *bgpserver.EventDispatcher satisfies it, and Reactor.removalEvents holds it as
// this interface so a test can observe every removal path's down.
type peerRemovalEvents interface {
	CapturePeerStateChange(peer *plugin.PeerInfo, state rpc.SessionState, reason string) func()
}

// peerRemoval is what removing one peer still owes once r.mu is released: the
// Cease that ends its session, then the down with reason rpc.ReasonPeerRemoved.
// Every path that deletes a peer from r.peers builds one (removalLocked) and
// hands it to completeRemovals, so every removal emits the same down: an API
// removal, a dynamic member the reactor drops, a reload's removal, and the
// undo of a reload's add.
type peerRemoval struct {
	// stop sends the Cease. A zero stop sends none: the undo of a reload's add
	// stops the peer itself, under r.mu.
	stop peerStop
	// removed is the identity the down carries.
	removed *plugin.PeerInfo
	// down owns the captured terminal delivery. completeRemovals MUST invoke
	// it exactly once, after the Cease and outside r.mu.
	down func()
}

// run sends the Cease alone, if the removal owes one. The caller MUST NOT hold
// r.mu. A removal is finished only by completeRemovals, which runs this first.
func (rm peerRemoval) run() {
	if rm.stop.peer != nil {
		rm.stop.run()
	}
}

// removalLocked issues the generation that ends peer's sessions and builds the
// identity its removal down carries. The caller MUST hold r.mu for writing and
// MUST have deleted the peer from r.peers in the same critical section, then
// MUST pass the result to completeRemovals once r.mu is released.
//
// SessionGeneration names the session this removal ends, as the FSM teardown's
// down does (endNegotiatedSession): a forwarding plugin stamps its departure
// with it, and the engine refuses a departure that names generation 0.
// The terminal recipients and role permissions are captured here before any
// concurrent graph publication can erase the peer's edges. The identity keeps
// the departure scoped to the removed lifetime, not a replacement at its address.
//
// The generation is issued here, under r.mu, rather than read: it is then above
// the generation every OPEN this peer validated was stamped with
// (Peer.validateOpen reads the clock), and no peer created again at this
// address or name can validate an OPEN until r.mu is released, so every OPEN of
// such a peer is stamped at or above it. A plugin that receives the down on its
// event loop and the new peer's OPEN on its callback loop orders the two by it
// (rpc.ValidateOpenInput.Generation): bgp-role frees the removed peer's learned
// role only when no newer OPEN stored it. A read would leave a peer that never
// reached Established on its previous session's generation, which an OPEN of
// its own already passed. The departure still covers every route the peer
// wrote: each carries an older generation.
func (r *Reactor) removalLocked(peer *Peer, stop peerStop) peerRemoval {
	settings := peer.Settings()
	peer.advanceForwardGeneration()
	removal := peerRemoval{stop: stop, removed: &plugin.PeerInfo{
		Address:           settings.Address,
		LocalAddress:      settings.LocalAddress,
		AddressStr:        peer.addrString,
		LocalAddressStr:   peer.localAddrString,
		Name:              settings.Name,
		GroupName:         settings.GroupName,
		LocalAS:           settings.LocalAS,
		PeerAS:            rpc.RemoteASOf(peer.PeerAS()), // guarded: a dynamic peer may still be resolving its ASN
		RouterID:          settings.RouterID,
		State:             peer.State().PluginState(),
		SessionGeneration: peer.forwardGeneration.Load(),
	}}
	if r.removalEvents != nil {
		// Capture allocates only on this cold removal path. The returned
		// delivery MUST run exactly once in completeRemovals after r.mu is free.
		removal.down = r.removalEvents.CapturePeerStateChange(removal.removed, rpc.SessionStateDown, rpc.ReasonPeerRemoved)
	}
	return removal
}

// completeRemovals finishes the removals removalLocked built. The caller MUST
// NOT hold r.mu: the Cease can block on a full send buffer, and plugin event
// delivery may block too.
//
// Each Cease is sent, then each down. The down is emitted unconditionally, not
// left to the FSM teardown, which fires only for an Established peer, so it
// also reaches a peer that was mid-reconnect at removal time. GR reads reason
// rpc.ReasonPeerRemoved to delete its per-peer ze_gr_* series and skip route
// retention, and bgp-role to free the peer's learned role.
//
// Recipients were captured by removalLocked under r.mu, so a concurrent peer
// change may already have republished the graph without losing this terminal
// delivery. Each captured down MUST run exactly once here, after its Cease and
// outside r.mu. The final publication reflects the current peer set.
func (r *Reactor) completeRemovals(removals ...peerRemoval) {
	for _, removal := range removals {
		removal.run()
	}
	for _, removal := range removals {
		if removal.down != nil {
			removal.down()
		}
	}
	r.mu.Lock()
	if r.deliveryPublished {
		r.publishDeliveryGraphLocked()
	}
	r.mu.Unlock()
}
