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
	OnPeerStateChange(peer *plugin.PeerInfo, state rpc.SessionState, reason string)
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
// with it, and the engine refuses a departure that names generation 0. The
// down reaches the processes first and the index then drops the peer, so the
// teardown's own down finds no receiver: without the generation here the route
// server's withdrawals were refused and the peer's routes stayed on every other
// peer.
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
	return peerRemoval{stop: stop, removed: &plugin.PeerInfo{
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
// Only then the index drops the peers' edges: the down finds its receivers
// through the index (PeerScopedProcs), so an index that no longer named the
// peer told no process it fed that it was gone, and the RIB, the route server
// and bgp-role kept its state. Guarded on the index being live for the same
// reason AddPeer is.
func (r *Reactor) completeRemovals(removals ...peerRemoval) {
	for _, removal := range removals {
		removal.run()
	}
	if r.removalEvents != nil {
		for _, removal := range removals {
			r.removalEvents.OnPeerStateChange(removal.removed, rpc.SessionStateDown, rpc.ReasonPeerRemoved)
		}
	}
	r.mu.Lock()
	if r.deliveryPublished {
		r.publishDeliveryGraphLocked()
	}
	r.mu.Unlock()
}
