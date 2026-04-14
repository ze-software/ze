// Design: plan/spec-iface-3-bgp-react.md — BGP reactions to interface events
// Overview: reactor.go — Reactor struct and lifecycle

package reactor

import (
	"encoding/json"
	"net"
	"net/netip"
	"strconv"

	"codeberg.org/thomas-mangin/ze/internal/core/events"
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
	unsubAdded := r.eventBus.Subscribe(events.NamespaceInterface, events.EventInterfaceAddrAdded, r.onInterfaceAddrAdded)
	unsubRemoved := r.eventBus.Subscribe(events.NamespaceInterface, events.EventInterfaceAddrRemoved, r.onInterfaceAddrRemoved)
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
		if _, emitErr := r.eventBus.Emit(events.NamespaceBGP, events.EventListenerReady, string(readyPayload)); emitErr != nil {
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
