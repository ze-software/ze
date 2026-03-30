// Design: docs/architecture/core-design.md — peer add/remove/lookup
// Overview: reactor.go — BGP reactor event loop and peer management

package reactor

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"time"
)

// parsePeerAddrToKey converts a peer address string (bare IP or "ip:port") to a
// netip.AddrPort map key. Bare IPs get DefaultBGPPort. Invalid strings return a
// zero AddrPort (which will simply not match any peer in the map).
func parsePeerAddrToKey(s string) netip.AddrPort {
	// Try as "ip:port" first.
	if ap, err := netip.ParseAddrPort(s); err == nil {
		return ap
	}
	// Try as bare IP with default port.
	if addr, err := netip.ParseAddr(s); err == nil {
		return netip.AddrPortFrom(addr, DefaultBGPPort)
	}
	return netip.AddrPort{}
}

// findPeerByAddr looks up a peer by address, trying default port first.
// Falls back to iterating peers by IP for non-standard port peers.
// Must be called with r.mu held (RLock or Lock).
func (r *Reactor) findPeerByAddr(addr netip.Addr) (*Peer, bool) {
	// Fast path: default port (standard BGP)
	if peer, ok := r.peers[PeerKeyFromAddrPort(addr, DefaultBGPPort)]; ok {
		return peer, true
	}
	// Slow path: search by IP (custom per-peer ports)
	for _, peer := range r.peers {
		if peer.Settings().Address == addr {
			return peer, true
		}
	}
	return nil, false
}

// findPeerKeyByAddr looks up a peer's map key and peer by address.
// Must be called with r.mu held.
func (r *Reactor) findPeerKeyByAddr(addr netip.Addr) (netip.AddrPort, *Peer, bool) {
	key := PeerKeyFromAddrPort(addr, DefaultBGPPort)
	if peer, ok := r.peers[key]; ok {
		return key, peer, true
	}
	for k, peer := range r.peers {
		if peer.Settings().Address == addr {
			return k, peer, true
		}
	}
	return netip.AddrPort{}, nil, false
}

// peerListenPort returns the port to listen on for a peer.
// Peers with custom ports get dedicated listeners; others share the global port.
func (r *Reactor) peerListenPort(s *PeerSettings) int {
	if s.Port != 0 && s.Port != DefaultBGPPort {
		return int(s.Port)
	}
	return r.config.Port
}

// AddPeer adds a peer to the reactor.
func (r *Reactor) AddPeer(settings *PeerSettings) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Normalize peer Address for consistent lookup (handles IPv4-mapped IPv6)
	// This ensures connections from 10.0.0.1 match peers configured as ::ffff:10.0.0.1
	settings.Address = settings.Address.Unmap()

	key := settings.PeerKey()
	if _, exists := r.peers[key]; exists {
		return ErrPeerExists
	}

	// Validate and normalize LocalAddress (only if set)
	if settings.LocalAddress.IsValid() {
		// Normalize IPv4-mapped IPv6 addresses (e.g., ::ffff:192.168.1.1 -> 192.168.1.1)
		settings.LocalAddress = settings.LocalAddress.Unmap()

		// Check self-referential (Address == LocalAddress)
		// Allow for loopback (127.0.0.0/8 or ::1) to support testing with next-hop self
		isLoopback := settings.Address.IsLoopback() && settings.LocalAddress.IsLoopback()
		if settings.Address == settings.LocalAddress && !isLoopback {
			return fmt.Errorf("peer %s: address cannot equal local-address", settings.Address)
		}

		// Check link-local IPv6 (requires zone ID, not portable)
		if settings.LocalAddress.Is6() && settings.LocalAddress.IsLinkLocalUnicast() {
			return fmt.Errorf("peer %s: link-local addresses not supported for local-address", settings.Address)
		}

		// Check address family mismatch (IPv4 peer with IPv6 LocalAddress or vice versa)
		// Note: Both Address and LocalAddress are already unmapped at this point
		if settings.Address.Is4() != settings.LocalAddress.Is4() {
			return fmt.Errorf("peer %s: address family mismatch (IPv4/IPv6)", settings.Address)
		}
	}

	peer := NewPeer(settings)
	peer.SetClock(r.clock)
	peer.SetDialer(r.dialer)
	peer.SetReactor(r)
	// Set message callback to forward raw bytes to reactor's message receiver
	peer.messageCallback = r.notifyMessageReceiver
	r.peers[key] = peer

	// Track peer's prefix demand for pool auto-sizing (AC-28).
	if r.fwdWeights != nil {
		r.fwdWeights.AddPeer(peer.peerAddrLabel(), totalPrefixMax(settings.PrefixMaximum), len(settings.PrefixMaximum))
	}

	// Update Prometheus gauges if metrics are configured.
	if r.rmetrics != nil {
		r.rmetrics.peersConfigured.Set(float64(len(r.peers)))
		r.rmetrics.peersAddedTotal.Inc()
		setPrefixConfigMetrics(r.rmetrics, settings.Address.String(), settings, r.clock.Now())
	}

	// Log staleness warning if prefix data is outdated.
	if IsPrefixDataStale(settings.PrefixUpdated, r.clock.Now()) {
		reactorLogger().Warn("prefix data is stale",
			"peer", settings.Address,
			"updated", settings.PrefixUpdated,
		)
	}

	// If reactor is running, start the peer and create listener if needed.
	// Active-only peers dial out and never accept inbound — skip listener.
	if r.running {
		if settings.LocalAddress.IsValid() && settings.Connection.IsPassive() {
			listenPort := r.peerListenPort(settings)
			lkey := net.JoinHostPort(settings.LocalAddress.String(), strconv.Itoa(listenPort))
			if existing, hasListener := r.listeners[lkey]; !hasListener {
				if err := r.startListenerForAddressPort(settings.LocalAddress, listenPort, settings.PeerKey()); err != nil {
					// Rollback peer addition
					delete(r.peers, key)
					return err
				}
			} else if settings.MD5Key != "" {
				// Listener exists but new peer has MD5 -- restart listener so
				// TCP_MD5SIG includes the new peer. Go's net.ListenConfig.Control
				// callback only fires at socket creation time.
				existing.Stop()
				waitCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				_ = existing.Wait(waitCtx)
				cancel()
				delete(r.listeners, lkey)
				var peerKey netip.AddrPort
				if settings.Port != 0 && settings.Port != DefaultBGPPort {
					peerKey = settings.PeerKey()
				}
				if err := r.startListenerForAddressPort(settings.LocalAddress, listenPort, peerKey); err != nil {
					delete(r.peers, key)
					return err
				}
			}
		}
		peer.StartWithContext(r.ctx)
	}

	return nil
}

// RemovePeer removes a peer from the reactor.
// Looks up by address, trying default port first then searching by IP.
func (r *Reactor) RemovePeer(addr netip.Addr) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Normalize address for consistent lookup (handles IPv4-mapped IPv6)
	addr = addr.Unmap()

	key, peer, exists := r.findPeerKeyByAddr(addr)
	if !exists {
		return ErrPeerNotFound
	}

	settings := peer.Settings()
	localAddr := settings.LocalAddress
	listenPort := r.peerListenPort(settings)

	// Stop peer if running
	peer.Stop()

	delete(r.peers, key)

	// Update Prometheus metrics if configured.
	if r.rmetrics != nil {
		r.rmetrics.peersConfigured.Set(float64(len(r.peers)))
		r.rmetrics.peersRemovedTotal.Inc()

		// Remove per-peer label entries so removed peers don't linger in /metrics.
		label := peer.peerAddrLabel()
		r.rmetrics.peerState.Delete(label)
		r.rmetrics.overflowItems.Delete(label)
		r.rmetrics.overflowRatio.Delete(label)
		r.rmetrics.sessionDuration.Delete(label)

		// Message counters have peer + type labels.
		for _, msgType := range []string{"update", "keepalive", "eor", "notification", "open", "route_refresh"} {
			r.rmetrics.peerMsgRecv.Delete(label, msgType)
			r.rmetrics.peerMsgSent.Delete(label, msgType)
		}

		// Session lifecycle counters (single-label: peer).
		r.rmetrics.sessionsEstablished.Delete(label)
		r.rmetrics.sessionFlaps.Delete(label)

		// Multi-label counters (peer + from/to or peer + code/subcode).
		// stateTransitions, notifSent, notifRecv have bounded cardinality
		// (FSM states x FSM states, notification codes x subcodes) so we
		// clean all observed combinations by iterating known states/codes.
		for _, from := range peerStateNames {
			for _, to := range peerStateNames {
				r.rmetrics.stateTransitions.Delete(label, from, to)
			}
		}
		// Notification codes: enumerate known BGP error codes (RFC 4271 + common).
		// This covers all standard notifications; exotic codes leave stale entries
		// but those are extremely rare in practice.
		for _, code := range notifCodeNames {
			for _, sub := range notifSubcodeNames {
				r.rmetrics.notifSent.Delete(label, code, sub)
				r.rmetrics.notifRecv.Delete(label, code, sub)
			}
		}
		// Wire-layer metrics (wireBytesRecv, etc.) are single-label.
		r.rmetrics.wireBytesRecv.Delete(label)
		r.rmetrics.wireBytesSent.Delete(label)
		r.rmetrics.wireReadErrors.Delete(label)
		r.rmetrics.wireWriteErrors.Delete(label)
		r.rmetrics.fwdCongestionEvents.Delete(label)
		r.rmetrics.fwdCongestionResume.Delete(label)
		r.rmetrics.prefixTeardownTotal.Delete(label)
		r.rmetrics.prefixStale.Delete(label)
		r.rmetrics.peerConnectAttempts.Delete(label)
		r.rmetrics.peerConnectAttemptSeconds.Delete(label)
		r.rmetrics.peerDialSeconds.Delete(label, "ok")
		r.rmetrics.peerDialSeconds.Delete(label, "fail")
		r.rmetrics.peerBackoffSeconds.Delete(label)
	}

	// Clean up source stats so disconnected peers don't accumulate in srcStats.
	if r.fwdPool != nil {
		r.fwdPool.RemoveSourceStats(peer.Settings().Address)
	}

	// Remove peer's prefix demand from pool auto-sizing (AC-28).
	if r.fwdWeights != nil {
		r.fwdWeights.RemovePeer(peer.peerAddrLabel())
	}

	// Check if any other peer uses this listener (same LocalAddress + port)
	if localAddr.IsValid() {
		stillUsed := false
		for _, p := range r.peers {
			ps := p.Settings()
			if ps.LocalAddress == localAddr && r.peerListenPort(ps) == listenPort {
				stillUsed = true
				break
			}
		}

		// Stop listener if no longer needed
		if !stillUsed {
			lkey := net.JoinHostPort(localAddr.String(), strconv.Itoa(listenPort))
			if listener, ok := r.listeners[lkey]; ok {
				listener.Stop()
				waitCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				_ = listener.Wait(waitCtx)
				cancel()
				delete(r.listeners, lkey)
			}
		}
	}

	return nil
}

// AddDynamicPeer adds a peer with the given configuration from the plugin API.
// Used by "bgp peer <ip> add" command for runtime peer management.
// LocalAS and RouterID default to reactor config if not specified.
func (r *Reactor) AddDynamicPeer(addr netip.Addr, tree map[string]any) error {
	// Inject remote.ip from selector (parsePeerFromTree requires it).
	if remote, ok := tree["remote"].(map[string]any); ok {
		remote["ip"] = addr.String()
	} else {
		tree["remote"] = map[string]any{"ip": addr.String()}
	}

	// Inject local.ip = "auto" if not set (parsePeerFromTree requires it).
	if local, ok := tree["local"].(map[string]any); ok {
		if _, hasIP := local["ip"]; !hasIP {
			local["ip"] = valAuto
		}
	} else {
		tree["local"] = map[string]any{"ip": valAuto}
	}

	name := addr.String()
	settings, err := parsePeerFromTree(name, tree, r.config.LocalAS, r.config.RouterID)
	if err != nil {
		return fmt.Errorf("dynamic peer %s: %w", name, err)
	}

	return r.AddPeer(settings)
}
