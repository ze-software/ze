// Design: docs/architecture/core-design.md — graceful restart state management
// RFC: rfc/short/rfc4724.md — Receiving Speaker procedures (Section 4.2)
// RFC: rfc/short/rfc9494.md — Long-Lived Graceful Restart (LLGR) procedures
// Overview: gr.go — GR plugin entry point, event dispatch, and capability storage
// Related: gr_llgr.go — LLGR capability types used by state machine for LLST tracking

package gr

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
// Removed when GR/LLGR completes (all EORs received) or all timers expire.
type grPeerState struct {
	// staleFamilies tracks which address families have stale routes.
	// Entries are removed as EORs arrive or on reconnect validation.
	staleFamilies map[string]bool

	// restartTimer fires after the peer's advertised Restart Time (GR phase).
	restartTimer *time.Timer

	// inLLGR is true when the peer has transitioned from GR to LLGR period.
	// RFC 9494: LLGR begins when GR restart-time expires.
	inLLGR bool

	// llgrFamilies tracks families currently in LLGR period with active LLST timers.
	// Only populated when inLLGR is true.
	llgrFamilies map[string]*time.Timer

	// llgrCap holds the LLGR capability from the peer's last OPEN.
	// Used during GR->LLGR transition to determine per-family LLST.
	llgrCap *llgrPeerCap
}

// grStateManager manages Graceful Restart and Long-Lived Graceful Restart
// state for all peers. It implements the Receiving Speaker procedures from
// RFC 4724 Section 4.2 and RFC 9494.
//
// Lifecycle:
//   - onSessionDown: creates GR state, marks families stale, starts restart timer
//   - handleTimerExpired: checks for LLGR; transitions or purges
//   - onSessionReestablished: validates new GR/LLGR caps, purges non-forwarding families
//   - onEORReceived: purges stale for family, stops LLST timer during LLGR
//   - LLST timer expiry: purges stale for that family; if last family, releases routes
type grStateManager struct {
	mu    sync.Mutex
	peers map[string]*grPeerState // peerAddr -> state

	// onTimerExpired is called when GR period ends without LLGR (purge all stale).
	onTimerExpired func(peerAddr string)

	// onLLGREnter is called per-family when transitioning from GR to LLGR.
	// The callback receives peer address, family, and LLST in seconds.
	onLLGREnter func(peerAddr, family string, llst uint32)

	// onLLGRFamilyExpired is called when an LLST timer expires for one family.
	onLLGRFamilyExpired func(peerAddr, family string)

	// onLLGREntryDone is called once after all families have entered LLGR.
	// Used to trigger readvertisement of updated routes.
	onLLGREntryDone func(peerAddr string)

	// onLLGRComplete is called when all LLGR families have expired or completed.
	onLLGRComplete func(peerAddr string)
}

// newGRStateManager creates a GR state manager.
// onExpired is called when a peer's restart timer fires and no LLGR is available.
func newGRStateManager(onExpired func(peerAddr string)) *grStateManager {
	return &grStateManager{
		peers:          make(map[string]*grPeerState),
		onTimerExpired: onExpired,
	}
}

// peerActive returns true if the peer is in GR or LLGR.
func (m *grStateManager) peerActive(peerAddr string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.peers[peerAddr]
	return ok
}

// onSessionDown is called when a peer's session drops.
// cap is the peer's GR capability from the previous OPEN (nil if no GR).
// llgrCap is the peer's LLGR capability (nil if no LLGR).
// wasNotification is true if the session ended due to NOTIFICATION.
//
// RFC 4724 Section 4.2: On TCP failure for GR-capable peer, retain routes
// for AFI/SAFI in GR capability, mark as stale, and start Restart Time timer.
// RFC 9494: If restart-time=0 and LLGR negotiated, enter LLGR immediately.
// NOTIFICATION sessions use normal BGP procedures (no route retention).
func (m *grStateManager) onSessionDown(peerAddr string, cap *grPeerCap, llgrCap *llgrPeerCap, wasNotification bool) bool {
	m.mu.Lock()

	// No GR capability or NOTIFICATION -> standard BGP (no route retention)
	if cap == nil || wasNotification {
		m.clearPeerLocked(peerAddr)
		m.mu.Unlock()
		return false
	}

	// RFC 4724 Section 4.2: consecutive restart -- delete previously stale
	m.clearPeerLocked(peerAddr)

	// Build stale family set from GR capability
	staleFamilies := make(map[string]bool, len(cap.Families))
	for _, f := range cap.Families {
		staleFamilies[f.Family] = true
	}

	if len(staleFamilies) == 0 {
		m.mu.Unlock()
		return false
	}

	state := &grPeerState{
		staleFamilies: staleFamilies,
		llgrCap:       llgrCap,
	}

	// RFC 9494: If restart-time=0 and LLGR negotiated, skip GR and enter LLGR immediately
	if cap.RestartTime == 0 && llgrCap != nil && len(llgrCap.Families) > 0 {
		m.peers[peerAddr] = state
		pending := m.enterLLGRLocked(peerAddr, state)
		m.mu.Unlock()
		pending.fire()
		return true
	}

	// Start GR restart timer
	restartDuration := time.Duration(cap.RestartTime) * time.Second
	state.restartTimer = time.AfterFunc(restartDuration, func() {
		m.handleTimerExpired(peerAddr)
	})

	m.peers[peerAddr] = state
	m.mu.Unlock()
	return true
}

// onSessionReestablished is called when a GR/LLGR-active peer reconnects.
// Returns the list of families whose stale routes should be purged immediately.
//
// RFC 4724 Section 4.2:
//   - No GR capability in new OPEN -> purge all stale routes
//   - AFI/SAFI missing from new GR -> purge stale for that family
//   - F-bit=0 for AFI/SAFI -> purge stale for that family
//   - F-bit=1 -> keep stale routes until EOR
//
// RFC 9494: During LLGR, also check new LLGR capability.
//   - If both GR and LLGR caps missing -> delete all stale
//   - If F-bit clear or family missing in new LLGR -> delete stale for that family
func (m *grStateManager) onSessionReestablished(peerAddr string, newCap *grPeerCap, newLLGRCap *llgrPeerCap) []string {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, ok := m.peers[peerAddr]
	if !ok {
		return nil // No GR/LLGR state -- nothing to do
	}

	// Stop GR restart timer if still running
	if state.restartTimer != nil {
		state.restartTimer.Stop()
		state.restartTimer = nil
	}

	// Stop all LLST timers if in LLGR
	m.stopLLSTTimersLocked(state)

	// No GR cap in new OPEN -> purge all
	if newCap == nil {
		purged := m.allStaleFamiliesLocked(state)
		m.deletePeerLocked(peerAddr)
		return purged
	}

	// Build lookup: families with F-bit=1 in new GR capability
	newForwarding := make(map[string]bool, len(newCap.Families))
	for _, f := range newCap.Families {
		if f.ForwardState {
			newForwarding[f.Family] = true
		}
	}

	// RFC 9494: Also check LLGR capability F-bits during LLGR reconnect
	if state.inLLGR && newLLGRCap != nil {
		for _, f := range newLLGRCap.Families {
			if f.ForwardState {
				newForwarding[f.Family] = true
			}
		}
	}

	// Check each stale family against new capabilities
	var purged []string
	for family := range state.staleFamilies {
		if !newForwarding[family] {
			purged = append(purged, family)
			delete(state.staleFamilies, family)
		}
	}

	// Update LLGR cap for potential future LLGR cycle
	state.llgrCap = newLLGRCap
	state.inLLGR = false

	// If all families purged, GR/LLGR is complete
	if len(state.staleFamilies) == 0 {
		m.deletePeerLocked(peerAddr)
	}

	return purged
}

// onEORReceived is called when End-of-RIB is received from a GR/LLGR-active peer.
// Returns true if stale routes for this family should be purged.
//
// RFC 4724 Section 4.2: On EOR receipt, immediately remove stale routes for that family.
// RFC 9494: During LLGR, also stop the LLST timer for that family.
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

	// Stop LLST timer for this family if in LLGR
	if state.inLLGR {
		if timer, ok := state.llgrFamilies[family]; ok {
			timer.Stop()
			delete(state.llgrFamilies, family)
		}
	}

	// All families received EOR -> GR/LLGR complete
	if len(state.staleFamilies) == 0 {
		if state.restartTimer != nil {
			state.restartTimer.Stop()
		}
		m.stopLLSTTimersLocked(state)
		m.deletePeerLocked(peerAddr)
	}

	return true
}

// llgrPendingActions collects callbacks to fire after releasing the state manager lock.
// Prevents holding m.mu across blocking RPCs (dispatchCommand).
type llgrPendingActions struct {
	entries   []llgrEntryAction // onLLGREnter per family
	purged    []string          // onLLGRFamilyExpired per family
	complete  bool              // onLLGRComplete (all families done immediately)
	entryDone bool              // onLLGREntryDone (readvertisement trigger)
	peerAddr  string
	mgr       *grStateManager
}

type llgrEntryAction struct {
	family string
	llst   uint32
}

// fire invokes all pending callbacks outside the lock.
func (p *llgrPendingActions) fire() {
	for _, e := range p.entries {
		if p.mgr.onLLGREnter != nil {
			p.mgr.onLLGREnter(p.peerAddr, e.family, e.llst)
		}
	}
	for _, family := range p.purged {
		if p.mgr.onLLGRFamilyExpired != nil {
			p.mgr.onLLGRFamilyExpired(p.peerAddr, family)
		}
	}
	if p.complete && p.mgr.onLLGRComplete != nil {
		p.mgr.onLLGRComplete(p.peerAddr)
	}
	if p.entryDone && p.mgr.onLLGREntryDone != nil {
		p.mgr.onLLGREntryDone(p.peerAddr)
	}
}

// handleTimerExpired handles GR restart timer expiry for a peer.
// RFC 4724 Section 4.2: delete all stale routes from the peer.
// RFC 9494: If LLGR negotiated, transition to LLGR instead of purging.
func (m *grStateManager) handleTimerExpired(peerAddr string) {
	m.mu.Lock()

	state, ok := m.peers[peerAddr]
	if !ok {
		m.mu.Unlock()
		return
	}

	// RFC 9494: Check if LLGR is available for any stale family
	if state.llgrCap != nil && len(state.llgrCap.Families) > 0 {
		pending := m.enterLLGRLocked(peerAddr, state)
		m.mu.Unlock()
		pending.fire()
		return
	}

	// No LLGR: standard GR expiry -- purge all stale
	m.clearPeerLocked(peerAddr)
	m.mu.Unlock()

	logger().Info("GR restart timer expired, purging stale routes", "peer", peerAddr)
	if m.onTimerExpired != nil {
		m.onTimerExpired(peerAddr)
	}
}

// enterLLGRLocked transitions a peer from GR to LLGR period.
// Must be called with m.mu held. Returns pending callbacks to fire after unlock.
// RFC 9494: Start per-family LLST timers, collect callback actions per family.
// Families without LLST (LLST=0) or not in the LLGR cap are purged immediately.
func (m *grStateManager) enterLLGRLocked(peerAddr string, state *grPeerState) *llgrPendingActions {
	state.inLLGR = true
	state.restartTimer = nil
	state.llgrFamilies = make(map[string]*time.Timer)

	pending := &llgrPendingActions{peerAddr: peerAddr, mgr: m}

	// Build LLST lookup from LLGR capability
	llstByFamily := make(map[string]uint32, len(state.llgrCap.Families))
	for _, f := range state.llgrCap.Families {
		if f.LLST > 0 {
			llstByFamily[f.Family] = f.LLST
		}
	}

	// Process each stale family
	for family := range state.staleFamilies {
		llst, hasLLGR := llstByFamily[family]
		if !hasLLGR {
			// Family not in LLGR cap or LLST=0 -> purge immediately
			pending.purged = append(pending.purged, family)
			continue
		}

		// Start per-family LLST timer with ownership guard.
		// Capture state pointer so stale callbacks from a previous GR cycle
		// can detect they no longer own the peer's state (consecutive restart).
		fam := family // capture for closure
		owner := state
		timer := time.AfterFunc(time.Duration(llst)*time.Second, func() {
			m.handleLLSTExpired(peerAddr, fam, owner)
		})
		state.llgrFamilies[family] = timer

		// Collect callback action (fired after unlock)
		pending.entries = append(pending.entries, llgrEntryAction{family: family, llst: llst})
	}

	// Remove purged families from stale tracking
	for _, family := range pending.purged {
		delete(state.staleFamilies, family)
	}

	// If no families entered LLGR, complete immediately
	if len(state.llgrFamilies) == 0 {
		m.deletePeerLocked(peerAddr)
		pending.complete = true
	} else {
		pending.entryDone = true
	}

	logger().Info("entered LLGR period",
		"peer", peerAddr,
		"llgr-families", len(state.llgrFamilies),
		"purged-families", len(pending.purged))

	return pending
}

// handleLLSTExpired handles LLST timer expiry for a specific family.
// RFC 9494: Delete stale routes for that family. If last family, release all.
// The owner parameter guards against stale callbacks from a previous GR cycle:
// if a consecutive restart replaced the peer's state, this callback is a no-op.
func (m *grStateManager) handleLLSTExpired(peerAddr, family string, owner *grPeerState) {
	m.mu.Lock()

	state, ok := m.peers[peerAddr]
	if !ok || state != owner {
		m.mu.Unlock()
		return
	}

	delete(state.staleFamilies, family)
	delete(state.llgrFamilies, family)

	lastFamily := len(state.llgrFamilies) == 0
	if lastFamily {
		m.deletePeerLocked(peerAddr)
	}

	m.mu.Unlock()

	logger().Info("LLST timer expired", "peer", peerAddr, "family", family, "last", lastFamily)

	if m.onLLGRFamilyExpired != nil {
		m.onLLGRFamilyExpired(peerAddr, family)
	}
	if lastFamily && m.onLLGRComplete != nil {
		m.onLLGRComplete(peerAddr)
	}
}

// clearPeerLocked removes GR/LLGR state for a peer, stopping all active timers.
// Must be called with m.mu held.
func (m *grStateManager) clearPeerLocked(peerAddr string) {
	if state, ok := m.peers[peerAddr]; ok {
		if state.restartTimer != nil {
			state.restartTimer.Stop()
		}
		m.stopLLSTTimersLocked(state)
		delete(m.peers, peerAddr)
	}
}

// deletePeerLocked removes the peer from the map without stopping timers.
// Used when timers have already been stopped individually.
// Must be called with m.mu held.
func (m *grStateManager) deletePeerLocked(peerAddr string) {
	delete(m.peers, peerAddr)
}

// stopLLSTTimersLocked stops all active LLST timers for a peer.
// Must be called with m.mu held.
func (m *grStateManager) stopLLSTTimersLocked(state *grPeerState) {
	for _, timer := range state.llgrFamilies {
		timer.Stop()
	}
	state.llgrFamilies = nil
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
