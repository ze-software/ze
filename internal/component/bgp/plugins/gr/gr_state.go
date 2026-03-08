// Design: docs/architecture/core-design.md — graceful restart state management
// RFC: rfc/short/rfc4724.md — Receiving Speaker procedures (Section 4.2)
// Overview: gr.go — GR plugin entry point, event dispatch, and capability storage

package bgp_gr

import (
	"sync"
	"time"
)

// grPeerCap holds the GR capability data extracted from a peer's OPEN message.
// This is a simplified representation of what we parse from the JSON event.
type grPeerCap struct {
	// RestartTime is the peer's advertised restart time in seconds (0-4095).
	RestartTime uint16
	// Families lists the AFI/SAFI pairs with their forwarding state (F-bit).
	Families []grCapFamily
}

// grCapFamily represents one AFI/SAFI entry in a peer's GR capability.
type grCapFamily struct {
	Family       string // "ipv4/unicast", "ipv6/unicast", etc.
	ForwardState bool   // F-bit: peer preserved forwarding state
}

// grPeerState holds the Graceful Restart state for a single peer during restart.
// Created when a GR-capable peer's session drops (TCP failure, not NOTIFICATION).
// Removed when GR completes (all EORs received) or restart timer expires.
type grPeerState struct {
	// staleFamilies tracks which address families have stale routes.
	// Entries are removed as EORs arrive or on reconnect validation.
	staleFamilies map[string]bool

	// restartTimer fires after the peer's advertised Restart Time.
	restartTimer *time.Timer
}

// grStateManager manages Graceful Restart state for all peers.
// It implements the Receiving Speaker procedures from RFC 4724 Section 4.2.
//
// Lifecycle:
//   - onSessionDown: creates GR state, marks families stale, starts restart timer
//   - onSessionReestablished: validates new GR capability, purges non-forwarding families
//   - onEORReceived: purges stale routes for the family, completes GR when all EORs received
//   - Timer expiry: purges all stale routes, removes GR state
type grStateManager struct {
	mu    sync.Mutex
	peers map[string]*grPeerState // peerAddr -> state

	// onTimerExpired is called when a peer's restart timer fires.
	// The callback receives the peer address and should trigger route purge.
	onTimerExpired func(peerAddr string)
}

// newGRStateManager creates a GR state manager.
// onExpired is called when a peer's restart timer fires (must be goroutine-safe).
func newGRStateManager(onExpired func(peerAddr string)) *grStateManager {
	return &grStateManager{
		peers:          make(map[string]*grPeerState),
		onTimerExpired: onExpired,
	}
}

// peerActive returns true if the peer is in GR (has stale routes being retained).
func (m *grStateManager) peerActive(peerAddr string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.peers[peerAddr]
	return ok
}

// onSessionDown is called when a peer's session drops.
// cap is the peer's GR capability from the previous OPEN (nil if no GR).
// wasNotification is true if the session ended due to NOTIFICATION.
//
// RFC 4724 Section 4.2: On TCP failure for GR-capable peer, retain routes
// for AFI/SAFI in GR capability, mark as stale, and start Restart Time timer.
// NOTIFICATION sessions use normal BGP procedures (no route retention).
func (m *grStateManager) onSessionDown(peerAddr string, cap *grPeerCap, wasNotification bool) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	// No GR capability or NOTIFICATION → standard BGP (no route retention)
	if cap == nil || wasNotification {
		m.clearPeerLocked(peerAddr)
		return false
	}

	// RFC 4724 Section 4.2: consecutive restart — delete previously stale
	m.clearPeerLocked(peerAddr)

	// Build stale family set from GR capability
	staleFamilies := make(map[string]bool, len(cap.Families))
	for _, f := range cap.Families {
		staleFamilies[f.Family] = true
	}

	if len(staleFamilies) == 0 {
		return false
	}

	// Start restart timer
	restartDuration := time.Duration(cap.RestartTime) * time.Second
	timer := time.AfterFunc(restartDuration, func() {
		m.handleTimerExpired(peerAddr)
	})

	m.peers[peerAddr] = &grPeerState{
		staleFamilies: staleFamilies,
		restartTimer:  timer,
	}

	return true
}

// onSessionReestablished is called when a GR-active peer reconnects.
// Returns the list of families whose stale routes should be purged immediately.
//
// RFC 4724 Section 4.2:
//   - No GR capability in new OPEN → purge all stale routes
//   - AFI/SAFI missing from new GR → purge stale for that family
//   - F-bit=0 for AFI/SAFI → purge stale for that family
//   - F-bit=1 → keep stale routes until EOR
func (m *grStateManager) onSessionReestablished(peerAddr string, newCap *grPeerCap) []string {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, ok := m.peers[peerAddr]
	if !ok {
		return nil // No GR state — nothing to do
	}

	// Stop restart timer — peer reconnected in time
	if state.restartTimer != nil {
		state.restartTimer.Stop()
		state.restartTimer = nil
	}

	// No GR cap in new OPEN → purge all
	if newCap == nil {
		purged := m.allStaleFamiliesLocked(state)
		delete(m.peers, peerAddr)
		return purged
	}

	// Build lookup: families with F-bit=1 in new capability
	newForwarding := make(map[string]bool, len(newCap.Families))
	for _, f := range newCap.Families {
		if f.ForwardState {
			newForwarding[f.Family] = true
		}
	}

	// Check each stale family against new capability
	var purged []string
	for family := range state.staleFamilies {
		if !newForwarding[family] {
			purged = append(purged, family)
			delete(state.staleFamilies, family)
		}
	}

	// If all families purged, GR is complete
	if len(state.staleFamilies) == 0 {
		delete(m.peers, peerAddr)
	}

	return purged
}

// onEORReceived is called when End-of-RIB is received from a GR-active peer.
// Returns true if stale routes for this family should be purged.
//
// RFC 4724 Section 4.2: On EOR receipt, immediately remove routes from the
// peer that are still marked as stale for that address family.
func (m *grStateManager) onEORReceived(peerAddr, family string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, ok := m.peers[peerAddr]
	if !ok {
		return false
	}

	if !state.staleFamilies[family] {
		return false
	}

	delete(state.staleFamilies, family)

	// All families received EOR → GR complete
	if len(state.staleFamilies) == 0 {
		if state.restartTimer != nil {
			state.restartTimer.Stop()
		}
		delete(m.peers, peerAddr)
	}

	return true
}

// handleTimerExpired handles restart timer expiry for a peer.
// RFC 4724 Section 4.2: delete all stale routes from the peer.
func (m *grStateManager) handleTimerExpired(peerAddr string) {
	m.mu.Lock()
	m.clearPeerLocked(peerAddr)
	m.mu.Unlock()

	logger().Info("GR restart timer expired, purging stale routes", "peer", peerAddr)
	if m.onTimerExpired != nil {
		m.onTimerExpired(peerAddr)
	}
}

// clearPeerLocked removes GR state for a peer, stopping any active timer.
// Must be called with m.mu held.
func (m *grStateManager) clearPeerLocked(peerAddr string) {
	if state, ok := m.peers[peerAddr]; ok {
		if state.restartTimer != nil {
			state.restartTimer.Stop()
		}
		delete(m.peers, peerAddr)
	}
}

// allStaleFamiliesLocked returns all stale families for a peer.
// Must be called with m.mu held.
func (m *grStateManager) allStaleFamiliesLocked(state *grPeerState) []string {
	families := make([]string, 0, len(state.staleFamilies))
	for family := range state.staleFamilies {
		families = append(families, family)
	}
	return families
}
