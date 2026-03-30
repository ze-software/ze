// Design: plan/spec-iface-3-bgp-react.md — BGP reactions to interface events
// Overview: reactor.go — Reactor struct and lifecycle
// Related: reactor_bus.go — Bus subscription infrastructure

package reactor

import (
	"encoding/json"
	"net"
	"net/netip"
	"strconv"

	"codeberg.org/thomas-mangin/ze/internal/component/iface"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

// RegisterInterfaceHandler registers a Bus handler for interface events.
// Must be called before StartWithContext.
//
// When an address is added, the reactor starts a listener on that address
// if any peer has a matching LocalAddress. When an address is removed,
// the reactor stops the listener for that address.
func (r *Reactor) RegisterInterfaceHandler() error {
	return r.OnBusEvent(iface.TopicPrefix, r.handleInterfaceEvent)
}

// handleInterfaceEvent dispatches interface Bus events to specific handlers.
// Runs inside the Bus's per-consumer delivery goroutine.
// MUST NOT hold reactor.mu (deadlock risk with publishBusNotification).
func (r *Reactor) handleInterfaceEvent(ev ze.Event) {
	switch ev.Topic {
	case iface.TopicAddrAdded:
		r.handleAddrAdded(ev)
	case iface.TopicAddrRemoved:
		r.handleAddrRemoved(ev)
	}
	// TopicCreated, TopicDeleted, TopicUp, TopicDown: no BGP action needed yet.
	// Future: TopicDown could mark peers for reconnection.
}

// handleAddrAdded starts a listener when an address matching a peer's
// LocalAddress appears.
func (r *Reactor) handleAddrAdded(ev ze.Event) {
	var payload iface.AddrPayload
	if err := json.Unmarshal(ev.Payload, &payload); err != nil {
		reactorLogger().Debug("iface: unmarshal addr event", "error", err)
		return
	}

	addr, err := netip.ParseAddr(payload.Address)
	if err != nil {
		reactorLogger().Debug("iface: parse address", "address", payload.Address, "error", err)
		return
	}

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
		"address", payload.Address, "unit", payload.Unit, "peers", len(matchingPeers))

	// Start listener for this address. startListenerForAddressPort is idempotent
	// (returns nil if already listening).
	r.mu.Lock()
	port := r.config.Port
	if err := r.startListenerForAddressPort(addr, port, netip.AddrPort{}); err != nil {
		reactorLogger().Error("iface: start listener failed",
			"address", payload.Address, "port", port, "error", err)
	}
	r.mu.Unlock()
}

// handleAddrRemoved stops the listener for an address that was removed.
func (r *Reactor) handleAddrRemoved(ev ze.Event) {
	var payload iface.AddrPayload
	if err := json.Unmarshal(ev.Payload, &payload); err != nil {
		reactorLogger().Debug("iface: unmarshal addr event", "error", err)
		return
	}

	addr, err := netip.ParseAddr(payload.Address)
	if err != nil {
		return
	}

	r.mu.Lock()
	port := r.config.Port
	// Use net.JoinHostPort for consistent key format with startListenerForAddressPort,
	// which also uses net.JoinHostPort. This is critical for IPv6 addresses where
	// JoinHostPort wraps the address in brackets: "[::1]:179".
	lkey := net.JoinHostPort(addr.String(), strconv.Itoa(port))
	listener, exists := r.listeners[lkey]
	if exists {
		reactorLogger().Info("iface: address removed, stopping listener",
			"address", payload.Address, "unit", payload.Unit)
		listener.Stop()
		delete(r.listeners, lkey)
	}
	r.mu.Unlock()
}
