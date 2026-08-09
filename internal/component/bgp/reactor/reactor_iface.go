// Design: docs/architecture/bgp/interface-event-reactions.md -- BGP reactions to interface events
// RFC: rfc/short/rfc2545.md — the Section 3 link-local condition is re-settled on an address event
// Overview: reactor.go — Reactor struct and lifecycle

package reactor

import (
	"encoding/json"
	"net"
	"net/netip"
	"slices"
	"strconv"

	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/internal/core/events"
	ifaceevents "github.com/ze-software/ze/internal/core/iface/events"
	"github.com/ze-software/ze/internal/core/network"
)

// interfaceAddrPayload is the JSON payload emitted by the interface monitor
// for (interface, addr-added) and (interface, addr-removed). The reactor
// parses it to discover the local address that needs a listener.
type interfaceAddrPayload struct {
	Name         string `json:"name"`
	Unit         int    `json:"unit"`
	Index        int    `json:"index"`
	Address      string `json:"address"`
	PrefixLength int    `json:"prefix-length"`
	Family       string `json:"family"`
}

// bgpListenerReadyPayload is the JSON payload emitted by the reactor on
// (bgp, listener-ready). Iface migration consumers wait for this signal
// before tearing down the old address.
type bgpListenerReadyPayload struct {
	Address string `json:"address"`
}

// SubscribeInterfaceEvents registers EventBus handlers for the interface
// events the reactor cares about. Replaces the legacy OnBusEvent prefix
// subscription that lived in reactor_bus.go. Must be called after
// SetEventBus and before StartWithContext.
//
// The interface monitor publishes nine event types in the (interface, *)
// namespace; the reactor only acts on addr-added and addr-removed today.
// Other events (created, up, down, dhcp-*, rollback) have no BGP-side
// reaction yet but the subscription points are documented for future
// handlers.
func (r *Reactor) SubscribeInterfaceEvents() {
	if r.eventBus == nil {
		return
	}
	unsubAdded := r.eventBus.Subscribe(ifaceevents.Namespace, ifaceevents.EventAddrAdded, events.AsString(r.onInterfaceAddrAdded))
	unsubRemoved := r.eventBus.Subscribe(ifaceevents.Namespace, ifaceevents.EventAddrRemoved, events.AsString(r.onInterfaceAddrRemoved))
	r.eventBusUnsubs = append(r.eventBusUnsubs, unsubAdded, unsubRemoved)
}

// onInterfaceAddrAdded is the EventBus handler for (interface, addr-added).
// Runs synchronously inside the EventBus delivery path; MUST NOT hold
// reactor.mu (deadlock risk with the listener startup path).
func (r *Reactor) onInterfaceAddrAdded(payload string) {
	var p interfaceAddrPayload
	if err := json.Unmarshal([]byte(payload), &p); err != nil {
		reactorLogger().Debug("iface: unmarshal addr-added", "error", err)
		return
	}
	r.handleAddrAddedPayload(p)
}

// onInterfaceAddrRemoved is the EventBus handler for (interface, addr-removed).
func (r *Reactor) onInterfaceAddrRemoved(payload string) {
	var p interfaceAddrPayload
	if err := json.Unmarshal([]byte(payload), &p); err != nil {
		reactorLogger().Debug("iface: unmarshal addr-removed", "error", err)
		return
	}
	r.handleAddrRemovedPayload(p)
}

// refreshPeerLinkScopes re-reads the host interface table once and re-settles
// RFC 2545 Section 3's inclusion condition for every peer that has a live
// forwarding snapshot.
//
// Section 3 decides the next-hop wire form against the subnets this host is
// attached to, and the snapshot that answers it is built at session
// establishment and at a config reload (link_scope.go). An address added to or
// removed from an interface OTHER than the one carrying the session moves that
// answer while the TCP connection survives, so without this the speaker keeps
// appending a link-local Section 3 now forbids, or keeps omitting one it now
// requires.
//
// This binds the FORM of every advertisement made after the event. Section 3
// constrains what a speaker "shall advertise ... in the Network Address of Next
// Hop field", which is the act of advertising; it states no obligation to
// re-advertise a route already sent, so no re-advertisement is triggered here.
//
// It runs synchronously on the EventBus delivery goroutine and MUST NOT hold
// reactor.mu while refreshing, so the peer list is copied under the read lock and
// the lock is released before any peer is touched. The kernel is read once for
// the whole fan-out rather than once per peer, and a peer whose scope already
// holds that exact table is left alone (see the loop).
func (r *Reactor) refreshPeerLinkScopes() {
	r.mu.RLock()
	peers := make([]*Peer, 0, len(r.peers))
	for _, peer := range r.peers {
		peers = append(peers, peer)
	}
	r.mu.RUnlock()

	if len(peers) == 0 {
		return
	}

	connected := network.ConnectedPrefixes()
	for _, peer := range peers {
		// An interface burst delivers one event per address, and the kernel already
		// holds every address of the burst by the time the first event is delivered.
		// Events 2..N therefore read a table identical to the one this peer's scope
		// was built from, and rebuilding on an identical table allocates a linkScope
		// and a forwarding-facts snapshot per peer per event to reach the same
		// answer. Section 3's condition is decided by the table alone, so an equal
		// table decides it equally.
		//
		// A peer that has never read a table has a nil scope, so the comparison is
		// false and it always rebuilds. A reordered table is not equal and rebuilds
		// too, which costs the saving rather than the answer.
		//
		// This guard belongs here and NOT in refreshForwardFactsIfLiveFrom
		// (peer_forward_facts.go). That function is also the settings-apply path
		// (peer_settings_apply.go), which changes inputs the interface table says
		// nothing about, and must rebuild every time.
		if scope := peer.llScope.Load(); scope != nil && slices.Equal(scope.connected, connected) {
			continue
		}
		peer.refreshForwardFactsIfLiveFrom(connected)
	}
}

// handleAddrAddedPayload starts a listener when an address matching a
// peer's LocalAddress appears. On success it emits (bgp, listener-ready)
// so iface migration consumers can complete their make-before-break.
func (r *Reactor) handleAddrAddedPayload(p interfaceAddrPayload) {
	addr, err := netip.ParseAddr(p.Address)
	if err != nil {
		reactorLogger().Debug("iface: parse address", "address", p.Address, "error", err)
		return
	}
	addr = addr.Unmap()

	// RFC 2545 Section 3: the new address changes which subnets this host is
	// attached to, for every peer and not only the ones bound to it.
	r.refreshPeerLinkScopes()

	// Find peers whose LocalAddress matches.
	r.mu.RLock()
	var matchingPeers []*Peer
	for _, peer := range r.peers {
		if peer.settings.LocalAddress == addr {
			matchingPeers = append(matchingPeers, peer)
		}
	}
	r.mu.RUnlock()

	if len(matchingPeers) == 0 {
		return
	}

	reactorLogger().Info("iface: address added, starting listener",
		"address", p.Address, "unit", p.Unit, "peers", len(matchingPeers))

	// Start listener for this address. startListenerForAddressPort is
	// idempotent (returns nil if already listening).
	r.mu.Lock()
	port := r.config.Port
	startErr := r.startListenerForAddressPort(addr, port, netip.AddrPort{})
	r.mu.Unlock()

	if startErr != nil {
		reactorLogger().Error("iface: start listener failed",
			"address", p.Address, "port", port, "error", startErr)
		return
	}
	if r.eventBus != nil {
		readyPayload, _ := json.Marshal(bgpListenerReadyPayload{Address: p.Address})
		if _, emitErr := r.eventBus.Emit(bgpevents.Namespace, bgpevents.EventListenerReady, string(readyPayload)); emitErr != nil {
			reactorLogger().Debug("iface: emit listener-ready", "address", p.Address, "error", emitErr)
		}
	}
}

// handleAddrRemovedPayload stops the listener for an address that was removed.
func (r *Reactor) handleAddrRemovedPayload(p interfaceAddrPayload) {
	addr, err := netip.ParseAddr(p.Address)
	if err != nil {
		return
	}
	addr = addr.Unmap()

	// RFC 2545 Section 3: a removed address can end a shared subnet, for every
	// peer and not only the one bound to this address.
	r.refreshPeerLinkScopes()

	r.mu.Lock()
	port := r.config.Port
	// Use net.JoinHostPort for consistent key format with
	// startListenerForAddressPort, which also uses net.JoinHostPort. This
	// is critical for IPv6 addresses where JoinHostPort wraps the address
	// in brackets: "[::1]:179".
	lkey := net.JoinHostPort(addr.String(), strconv.Itoa(port))
	listener, exists := r.listeners[lkey]
	if exists {
		reactorLogger().Info("iface: address removed, stopping listener",
			"address", p.Address, "unit", p.Unit)
		listener.Stop()
		delete(r.listeners, lkey)
	}
	r.mu.Unlock()
}
