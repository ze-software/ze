// Design: docs/architecture/core-design.md — peer add/remove/lookup
// RFC: rfc/short/rfc4271.md — ManualStop seals a stopping reactor against a new peer
// RFC: rfc/short/rfc7911.md — a removed peer's Path Identifiers are released
// RFC: rfc/short/rfc8654.md — a peer's outgoing pool is sized for extended messages
// Overview: reactor.go — BGP reactor event loop and peer management

package reactor

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// envKeyTestPort names the BGP port every peer of this daemon dials, for the
// test infrastructure alone. It is not a YANG leaf and no operator sets it.
const envKeyTestPort = "ze.test.bgp.port"

var _ = env.MustRegister(env.EnvEntry{
	Key:         envKeyTestPort,
	Type:        "int",
	Default:     "",
	Description: "BGP listen port (test infrastructure)",
	Private:     true,
})

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
	if peer, ok := r.peers[peerKeyFromAddrPort(addr, DefaultBGPPort)]; ok {
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
	key := peerKeyFromAddrPort(addr, DefaultBGPPort)
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
//
// A peer naming connection > local > port gets a listener of its own on it; the
// rest share the daemon's port. The value read here is LocalPort and never Port,
// because Port is the REMOTE endpoint, the port Ze dials. Both leaves wrote Port
// until 2026-09-06, so a config naming both listened on the port of the peer
// rather than the port the operator asked for (peer_settings.go).
func (r *Reactor) peerListenPort(s *PeerSettings) int {
	if s.LocalPort != 0 && s.LocalPort != DefaultBGPPort {
		return int(s.LocalPort)
	}
	if r.config.Port != 0 {
		return r.config.Port
	}
	return DefaultBGPPort
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
	r.peerGeneration.Add(1)

	// Track peer's prefix demand for pool auto-sizing (AC-28).
	if r.fwdWeights != nil {
		r.fwdWeights.AddPeer(peer.peerAddrLabel(), totalPrefixMax(settings.PrefixMaximum), len(settings.PrefixMaximum))
	}

	// Register per-peer pool for steady-state micro-burst absorption.
	// Default to 4K (standard); re-registered with 64K in notifyPeerEstablished
	// if Extended Message (RFC 8654) is negotiated.
	if r.fwdPool != nil {
		r.fwdPool.registerOutgoingPool(fwdKey{peerAddr: key}, message.MaxMsgLen)
	}

	// Update Prometheus gauges if metrics are configured.
	if r.rmetrics != nil {
		r.rmetrics.peersConfigured.Set(float64(len(r.peers)))
		r.rmetrics.peersAddedTotal.Inc()
		setPrefixConfigMetrics(r.rmetrics, settings.Address.String(), settings, r.clock.Now())
	}

	// Raise / clear prefix-stale on the report bus based on PrefixUpdated.
	// Mirrors the existing Prometheus stale gauge but is operator-visible via
	// `ze show warnings` and the login banner.
	//
	// Lock ordering: this is called while r.mu is held by AddPeer's caller.
	// The report bus acquires its own internal mutex; reactor.mu -> report.mu
	// is the established ordering. The bus is a leaf component (no imports
	// of reactor), so no inversion is possible.
	// The oldest per-family date drives both surfaces, so the alarm stays
	// raised while any one family is stale.
	oldestUpdated := settings.OldestPrefixUpdated()
	raisePrefixStale(settings.Address.String(), oldestUpdated, r.clock.Now())

	// Log staleness warning if prefix data is outdated.
	if isPrefixDataStale(oldestUpdated, r.clock.Now()) {
		reactorLogger().Warn("prefix data is stale",
			"peer", settings.Address,
			"updated", oldestUpdated,
		)
	}

	// If reactor is running, start the peer and create listener if needed.
	// Active-only peers dial out and never accept inbound — skip listener.
	//
	// A stop that has begun stays sealed: reactor.go shuts the listeners and
	// marks every peer stopping under this same lock, so starting one here
	// would open a session nothing notifies and the cancel closes in silence
	// (RFC 4271 Section 8.2.2, ManualStop). The peer stays in r.peers, inert,
	// and the process is leaving.
	if r.running && !r.stopping {
		if settings.LocalAddress.IsValid() && settings.Connection.IsPassive() {
			listenPort := r.peerListenPort(settings)
			lkey := net.JoinHostPort(settings.LocalAddress.String(), strconv.Itoa(listenPort))
			if existing, hasListener := r.listeners[lkey]; !hasListener {
				if err := r.startListenerForAddressPort(settings.LocalAddress, listenPort); err != nil {
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
				if err := r.startListenerForAddressPort(settings.LocalAddress, listenPort); err != nil {
					delete(r.peers, key)
					return err
				}
			}
		}
		peer.StartWithContext(r.ctx)
	}

	// Republish the peer-to-process index, which this peer's attach blocks have
	// just changed (delivery_graph.go). Only once the index is live: the
	// startup load calls AddPeer once per peer, and StartWithContext publishes
	// once for the whole set rather than once per peer.
	if r.deliveryPublished {
		r.publishDeliveryGraphLocked()
	}

	return nil
}

// RemovePeer removes a peer from the reactor.
// Looks up by address, trying default port first then searching by IP.
func (r *Reactor) RemovePeer(addr netip.Addr) error {
	removed, err := r.doRemovePeer(addr)
	if err != nil {
		return err
	}
	// Notify plugins the peer was removed, AFTER releasing r.mu: plugin event
	// delivery may block, so it must not run under the reactor lock. Emitted
	// unconditionally here (not via the FSM teardown, which only fires for
	// Established peers) so it also reaches peers that were mid-reconnect at
	// removal time. GR uses reason rpc.ReasonPeerRemoved to delete its per-peer
	// ze_gr_* series and skip route retention.
	if removed != nil && r.eventDispatcher != nil {
		r.eventDispatcher.OnPeerStateChange(removed, rpc.SessionStateDown, rpc.ReasonPeerRemoved)
	}
	return nil
}

// doRemovePeer performs the locked peer-removal work and returns the removed
// peer's identity so RemovePeer can notify plugins after releasing r.mu.
func (r *Reactor) doRemovePeer(addr netip.Addr) (*plugin.PeerInfo, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Normalize address for consistent lookup (handles IPv4-mapped IPv6)
	addr = addr.Unmap()

	key, peer, exists := r.findPeerKeyByAddr(addr)
	if !exists {
		return nil, ErrPeerNotFound
	}

	settings := peer.Settings()
	localAddr := settings.LocalAddress
	listenPort := r.peerListenPort(settings)

	// Stop peer if running
	peer.Stop()

	// Release the AS-wide BGP Identifier claim synchronously, for the same
	// reason the reload-remove path does (reactor_api.go): Stop only cancels a
	// context, so without this the outgoing peer's claim outlives its removal
	// until its goroutine happens to be scheduled. A RemovePeer+AddPeer pair
	// (re-address, or a router-id move) would then meet its own stale claim and
	// answer the legitimate new session with Bad BGP Identifier, decided purely
	// by scheduling. The peer's own later release is a no-op that cannot touch a
	// new holder's entry (routerIDClaims.release checks holder.peer == p).
	peer.releaseRouterIDClaim()

	// End this peer's protocol event capture synchronously, for the same reason
	// and the same race as the claim release above: the capture file is named
	// for the peer address (capture_replay.go), so a RemovePeer+AddPeer pair
	// would otherwise open a second capture on a path the outgoing one still
	// holds, and the two would rotate each other's live file into the single
	// `.1` slot.
	r.closeCapturesForPeer(addr)

	// Clear any prefix-stale warning for this peer from the report bus.
	// Threshold warnings are cleared by Session.ClearReportedWarnings
	// during the session teardown defer in peer_run.go.
	clearPrefixStale(addr.String())

	if peer.health != nil {
		peer.health.stop()
	}

	// Drop the RFC 7911 Path Identifiers ze assigned to this peer's paths
	// (forward_path_id.go). Removal is the point where those paths are gone for
	// good: a peer that merely reconnects keeps its entries, so it re-announces
	// each path under the identifier the destinations already hold, which they
	// read as the replacement it is.
	fwdPathIDs.releaseSource(peer.SourceID())

	delete(r.peers, key)
	r.peerGeneration.Add(1)

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
		r.rmetrics.connectRetryCounter.Delete(label)

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
		r.rmetrics.attrSpanSpill.Delete(label)
		r.rmetrics.fwdCongestionEvents.Delete(label)
		r.rmetrics.fwdCongestionResume.Delete(label)
		r.rmetrics.prefixTeardownTotal.Delete(label)
		r.rmetrics.prefixStale.Delete(label)
		r.rmetrics.peerConnectAttempts.Delete(label)
		r.rmetrics.peerConnectAttemptSeconds.Delete(label)
		r.rmetrics.peerDialSeconds.Delete(label, "ok")
		r.rmetrics.peerDialSeconds.Delete(label, "fail")
		r.rmetrics.peerBackoffSeconds.Delete(label)
		r.rmetrics.captureDroppedEvents.Delete(label)
	}

	// Clean up source stats so disconnected peers don't accumulate in srcStats.
	if r.fwdPool != nil {
		r.fwdPool.removeSourceStats(peer.Settings().Address)
	}

	// Remove peer's prefix demand from pool auto-sizing (AC-28).
	if r.fwdWeights != nil {
		r.fwdWeights.RemovePeer(peer.peerAddrLabel())
	}

	// Unregister per-peer pool on session teardown.
	if r.fwdPool != nil {
		r.fwdPool.unregisterOutgoingPool(fwdKey{peerAddr: key})
	}

	// Check whether anything else still uses this listener. A dynamic group
	// counts: it accepts on the same socket, and no peer entry represents it.
	// Counting peers alone shuts the ear of a route server whose last static
	// peer leaves.
	if localAddr.IsValid() {
		claimants, _ := r.listenerClaimants(localAddr, listenPort)

		// Stop listener if no longer needed
		if claimants == 0 {
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

	// Republish the peer-to-process index without this peer's edges: a process
	// it fed must stop being fed for it (delivery_graph.go). Guarded on the
	// index being live for the same reason AddPeer is.
	if r.deliveryPublished {
		r.publishDeliveryGraphLocked()
	}

	// Build the removed peer's identity for the post-unlock plugin notification.
	removed := &plugin.PeerInfo{
		Address:         settings.Address,
		LocalAddress:    settings.LocalAddress,
		AddressStr:      peer.addrString,
		LocalAddressStr: peer.localAddrString,
		Name:            settings.Name,
		GroupName:       settings.GroupName,
		LocalAS:         settings.LocalAS,
		PeerAS:          peer.PeerAS(), // guarded: a dynamic peer may still be resolving its ASN
		RouterID:        settings.RouterID,
		State:           peer.State().PluginState(),
	}

	return removed, nil
}

// AddDynamicPeer adds a peer to the running reactor from a config tree the
// caller built, and starts it. `create bgp peer <address> asn <asn> ...` is the
// command that reaches it (handleBgpPeerAdd,
// internal/component/bgp/plugins/cmd/peer/create.go).
//
// The tree has the same shape as one peer's subtree in the configuration file,
// so parsePeerFromTree reads both and a leaf added there is honored here. The
// peer lives in the reactor alone: nothing is written to the configuration, and
// RemovePeer mirrors that on the way out.
//
// The local AS and the router ID default to the reactor's own when the tree
// states neither.
//
// The two leaves parsePeerFromTree REQUIRES are filled in here when the caller
// left them out: the remote address is the one the caller named, and the local
// address defaults to "auto". Both sit under `connection`, which is where the
// parser reads them. They were written at the TOP of the tree until 2026-09-06,
// where nothing reads them, so every call failed with "missing required
// connection > remote > ip" (ai/rules/principles.md).
func (r *Reactor) AddDynamicPeer(addr netip.Addr, tree map[string]any) error {
	conn := treeContainer(tree, "connection")
	treeContainer(conn, "remote")["ip"] = addr.String()

	local := treeContainer(conn, "local")
	if _, hasIP := local["ip"]; !hasIP {
		local["ip"] = valAuto
	}

	name := addr.String()
	settings, err := parsePeerFromTree(name, tree, r.config.LocalAS, r.config.RouterID)
	if err != nil {
		return fmt.Errorf("dynamic peer %s: %w", name, err)
	}

	// The test port override reaches a peer built HERE as well as one built by
	// the config loader, which applies it in applyPortOverride
	// (internal/component/bgp/config/peers.go). A peer is a peer whichever
	// route created it, and a runtime-created one that dialed 179 while every
	// configured peer dialed the harness port would reach no test server.
	if port, override := PortOverrideFromEnv(); override {
		settings.Port = port
	}

	return r.AddPeer(settings)
}

// PortOverrideFromEnv answers the BGP port every peer of this daemon dials,
// when the test infrastructure named one, and false when it did not.
//
// It is runtime-only and it is not a YANG leaf: the harness starts its mock
// speaker on an ephemeral port and tells the daemon which one through
// `ze.test.bgp.port`. It lives here, beside AddPeer, so the config loader and
// the runtime create path read ONE declaration of the key
// (ai/rules/principles.md).
func PortOverrideFromEnv() (uint16, bool) {
	value := env.Get(envKeyTestPort)
	if value == "" {
		return 0, false
	}
	port, err := strconv.ParseUint(value, 10, 16)
	if err != nil {
		return 0, false
	}
	return uint16(port), true //nolint:gosec // ParseUint bounded the value to 16 bits above.
}

// treeContainer answers the child map at key, creating it when the tree holds
// none.
//
// A value of another type at that key is REPLACED. The tree is built by the
// caller for this one call and never read from a file, so anything but a
// container there is a caller defect rather than operator data, and the parser
// below reports the leaf it could not find.
func treeContainer(tree map[string]any, key string) map[string]any {
	if child, ok := tree[key].(map[string]any); ok {
		return child
	}
	child := make(map[string]any)
	tree[key] = child
	return child
}
