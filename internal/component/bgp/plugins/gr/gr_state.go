// Design: docs/architecture/core-design.md — graceful restart state management
// RFC: rfc/short/rfc4724.md — Receiving Speaker procedures (Section 4.2)
// RFC: rfc/short/rfc9494.md — Long-Lived Graceful Restart (LLGR) procedures
// Overview: gr.go — GR plugin entry point, event dispatch, and capability storage
// Related: gr_llgr.go — LLGR capability types used by state machine for LLST tracking

package gr

import (
	"sync"
	"time"

	"github.com/ze-software/ze/internal/core/family"
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
	Family       family.Family // typed AFI/SAFI identity
	ForwardState bool          // F-bit: peer preserved forwarding state
}

// grPeerState holds the Graceful Restart state for a single peer during restart.
// Created when a GR-capable peer's session drops (TCP failure, not NOTIFICATION).
// Removed when GR/LLGR completes (all EORs received) or all timers expire.
type grPeerState struct {
	// staleFamilies tracks which address families have stale routes.
	// Entries are removed as EORs arrive or on reconnect validation.
	staleFamilies map[family.Family]bool

	// restartTimer fires after the peer's advertised Restart Time (GR phase).
	restartTimer *time.Timer

	// restartDeadline is the conventional GR deadline. Omitted GR families
	// enter LLGR at DOWN instead. Zero means no conventional GR remains.
	restartDeadline time.Time

	// inLLGR is true while at least one family has an LLGR timer.
	// Other families can still be in their conventional GR period.
	inLLGR bool

	// llgrFamilies tracks families currently in LLGR period with active LLST timers.
	// Only populated when inLLGR is true.
	llgrFamilies map[family.Family]*time.Timer

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
	onLLGREnter func(peerAddr string, fam family.Family, llst uint32)

	// onLLGRFamilyExpired is called when an LLST timer expires for one family.
	onLLGRFamilyExpired func(peerAddr string, fam family.Family)

	// onLLGREntryDone is called once after all families have entered LLGR.
	// Used to trigger per-family readvertisement of updated routes.
	// families contains the address families that entered LLGR (e.g., ipv4/unicast).
	onLLGREntryDone func(peerAddr string, families []family.Family)

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

// familyRetained answers the forwarding owner's query using the authoritative
// GR/LLGR family set. Safe for concurrent use; callers MUST NOT hold m.mu.
func (m *grStateManager) familyRetained(peerAddr string, fam family.Family) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	state := m.peers[peerAddr]
	return state != nil && state.staleFamilies[fam]
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
// llgrCap is the exchanged LLGR capability (nil if no enabled LLGR family).
// wasNotification is true if the session ended due to NOTIFICATION.
//
// RFC 4724 Section 4.2: On TCP failure for a GR-capable peer, retain GR families,
// mark them stale, and start the Restart Time timer.
// RFC 9494 Section 4.2: Omitted GR families with nonzero exchanged LLST enter
// LLGR immediately, as do listed families when Restart Time is zero.
// NOTIFICATION sessions use normal BGP procedures (no route retention).
func (m *grStateManager) onSessionDown(peerAddr string, cap *grPeerCap, llgrCap *llgrPeerCap, wasNotification bool) bool {
	activated, completeDown := m.onSessionDownDeferred(peerAddr, cap, llgrCap, wasNotification)
	if completeDown != nil {
		completeDown()
	}
	return activated
}

// onSessionDownDeferred prepares retention without firing transition callbacks.
// The caller MUST invoke the returned completion, when non-nil, AFTER dispatching
// the session-down sequence (purge the previous cycle's stale routes, retain,
// mark stale). Otherwise expiry can release routes before they are retained, or
// the sequence's purge-stale can delete routes just marked LLGR-stale.
//
// RFC 9494 Section 4.2: "After the session goes down, and before the session is
// re-established, the stale routes for an AFI/SAFI MUST be retained."
// The original GR deadline includes time spent dispatching the sequence.
func (m *grStateManager) onSessionDownDeferred(peerAddr string, cap *grPeerCap, llgrCap *llgrPeerCap, wasNotification bool) (bool, func()) {
	m.mu.Lock()

	// No GR capability or NOTIFICATION -> standard BGP (no route retention)
	if cap == nil || wasNotification {
		m.clearPeerLocked(peerAddr)
		m.mu.Unlock()
		return false, nil
	}

	// RFC 4724 Section 4.2: consecutive restart -- delete previously stale
	m.clearPeerLocked(peerAddr)

	// RFC 9494 Section 4.2: "If the Graceful Restart Capability that was received
	// does not list all AFIs/SAFIs supported by the session, then the GR Restart
	// Time shall be deemed zero for those AFIs/SAFIs that are not listed."
	downAt := time.Now()
	staleFamilies := make(map[family.Family]bool, len(cap.Families))
	for _, f := range cap.Families {
		staleFamilies[f.Family] = true
	}
	var immediate []family.Family
	if llgrCap != nil {
		for _, f := range llgrCap.Families {
			if staleFamilies[f.Family] {
				continue
			}
			if f.LLST == 0 {
				continue
			}
			staleFamilies[f.Family] = true
			immediate = append(immediate, f.Family)
		}
	}

	if len(staleFamilies) == 0 {
		m.mu.Unlock()
		return false, nil
	}

	state := &grPeerState{
		staleFamilies: staleFamilies,
		llgrCap:       llgrCap,
	}
	if len(cap.Families) > 0 {
		state.restartDeadline = downAt.Add(time.Duration(cap.RestartTime) * time.Second)
	}

	m.peers[peerAddr] = state
	m.mu.Unlock()
	return true, func() {
		var pending *llgrPendingActions
		if len(immediate) > 0 {
			m.mu.Lock()
			if m.peers[peerAddr] != state {
				m.mu.Unlock()
				return
			}
			// RFC 9494 Section 4.2. Retention and stale marking have completed;
			// omitted families start their LLST budget at the original DOWN.
			pending = m.enterLLGRLocked(peerAddr, state, immediate, downAt)
			m.mu.Unlock()
		}
		// RFC 9494 Section 4.2: "The interval for which they are retained is limited
		// by the sum of the Restart Time in the received Graceful Restart Capability
		// and the Long-Lived Stale Time in the received Long-Lived Graceful Restart
		// Capability." An immediate family's RPC MUST NOT delay a sibling's timer.
		m.startRestartTimer(peerAddr, state)
		if pending != nil {
			pending.fire()
		}
	}
}

// startRestartTimer completes session-down only after the RIB has marked routes
// stale. Safe for concurrent use; callers MUST NOT hold m.mu.
//
// RFC 4724 Section 4.2: "If the session does not get re-established within the
// "Restart Time" that the peer advertised previously, the Receiving Speaker MUST
// delete all the stale routes from the peer that it is retaining."
// The deadline remains anchored to the original session-down time.
func (m *grStateManager) startRestartTimer(peerAddr string, state *grPeerState) {
	m.mu.Lock()
	if m.peers[peerAddr] != state {
		m.mu.Unlock()
		return
	}
	if state.restartDeadline.IsZero() {
		m.mu.Unlock()
		return
	}

	if remaining := time.Until(state.restartDeadline); remaining > 0 {
		state.restartTimer = time.AfterFunc(remaining, func() {
			// RFC 4724 Section 4.2, RFC 9494 Section 4.2.
			m.handleTimerExpired(peerAddr, state)
		})
		m.mu.Unlock()
		return
	}
	m.mu.Unlock()

	// RFC 4724 Section 4.2. A zero or elapsed deadline expires synchronously,
	// after stale marking, rather than racing the session-down commands.
	m.handleTimerExpired(peerAddr, state)
}

// onSessionReestablished is called when a GR/LLGR-active peer reconnects.
// Returns the list of families whose stale routes should be purged immediately,
// and whether the peer was in LLGR state before reestablishment (for counter decrement).
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
func (m *grStateManager) onSessionReestablished(peerAddr string, newCap *grPeerCap, newLLGRCap *llgrPeerCap) ([]family.Family, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, ok := m.peers[peerAddr]
	if !ok {
		return nil, false // No GR/LLGR state -- nothing to do
	}

	wasInLLGR := state.inLLGR

	// Cancel callbacks that already started and cannot be stopped by Timer.Stop.
	state.restartDeadline = time.Time{}
	// Stop GR restart timer if still running.
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
		return purged, wasInLLGR
	}

	// Build lookup: families with F-bit=1 in new GR capability
	newForwarding := make(map[family.Family]bool, len(newCap.Families))
	for _, f := range newCap.Families {
		if f.ForwardState {
			newForwarding[f.Family] = true
		}
	}

	// RFC 9494: Also check LLGR capability F-bits during LLGR reconnect
	if wasInLLGR && newLLGRCap != nil {
		for _, f := range newLLGRCap.Families {
			if f.ForwardState {
				newForwarding[f.Family] = true
			}
		}
	}

	// Check each stale family against new capabilities
	var purged []family.Family
	for fam := range state.staleFamilies {
		if !newForwarding[fam] {
			purged = append(purged, fam)
			delete(state.staleFamilies, fam)
		}
	}

	// Update LLGR cap for potential future LLGR cycle
	state.llgrCap = newLLGRCap
	state.inLLGR = false

	// If all families purged, GR/LLGR is complete
	if len(state.staleFamilies) == 0 {
		m.deletePeerLocked(peerAddr)
	}

	return purged, wasInLLGR
}

// onEORReceived is called when End-of-RIB is received from a GR/LLGR-active peer.
// Returns true if stale routes for this family should be purged.
//
// RFC 4724 Section 4.2: On EOR receipt, immediately remove stale routes for that family.
// RFC 9494: During LLGR, also stop the LLST timer for that family.
func (m *grStateManager) onEORReceived(peerAddr string, fam family.Family) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, ok := m.peers[peerAddr]
	if !ok {
		return false
	}

	if !state.staleFamilies[fam] {
		return false
	}

	delete(state.staleFamilies, fam)

	// Stop LLST timer for this family if in LLGR
	if state.inLLGR {
		if timer, ok := state.llgrFamilies[fam]; ok {
			timer.Stop()
			delete(state.llgrFamilies, fam)
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
	purged    []family.Family   // onLLGRFamilyExpired per family
	complete  bool              // onLLGRComplete (all families done immediately)
	entryDone bool              // onLLGREntryDone (readvertisement trigger)
	peerAddr  string
	mgr       *grStateManager
}

type llgrEntryAction struct {
	family family.Family
	llst   uint32
}

// fire invokes all pending callbacks outside the lock.
// The collecting caller MUST release m.mu before calling fire.
func (p *llgrPendingActions) fire() {
	for _, e := range p.entries {
		if p.mgr.onLLGREnter != nil {
			p.mgr.onLLGREnter(p.peerAddr, e.family, e.llst)
		}
	}
	for _, fam := range p.purged {
		if p.mgr.onLLGRFamilyExpired != nil {
			p.mgr.onLLGRFamilyExpired(p.peerAddr, fam)
		}
	}
	if p.complete && p.mgr.onLLGRComplete != nil {
		p.mgr.onLLGRComplete(p.peerAddr)
	}
	if p.entryDone && p.mgr.onLLGREntryDone != nil {
		families := make([]family.Family, len(p.entries))
		for i, e := range p.entries {
			families[i] = e.family
		}
		p.mgr.onLLGREntryDone(p.peerAddr, families)
	}
}

// handleTimerExpired handles GR restart timer expiry for a peer.
// RFC 4724 Section 4.2: delete all stale routes from the peer.
// RFC 9494: If LLGR negotiated, transition to LLGR instead of purging.
// The owner must be the cycle that armed the callback: Timer.Stop cannot cancel
// a callback that already started and is waiting for m.mu.
func (m *grStateManager) handleTimerExpired(peerAddr string, owner *grPeerState) {
	m.mu.Lock()

	state, ok := m.peers[peerAddr]
	if !ok {
		m.mu.Unlock()
		return
	}
	if state != owner {
		m.mu.Unlock()
		return
	}
	if state.restartDeadline.IsZero() {
		m.mu.Unlock()
		return
	}

	// RFC 9494: Check if LLGR is available for any stale family
	if state.llgrCap != nil && len(state.llgrCap.Families) > 0 {
		// RFC 9494 Section 4.2. Already-running LLST families are not restarted.
		pending := m.enterLLGRLocked(peerAddr, state, m.allStaleFamiliesLocked(state), state.restartDeadline)
		state.restartDeadline = time.Time{}
		state.restartTimer = nil
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

// enterLLGRLocked transitions the selected families whose GR period has ended.
// Caller MUST hold m.mu and MUST fire the returned callbacks after unlocking.
// Existing family timers are preserved; deadline is the selected GR period's end.
// RFC 9494 Section 4.2: "For each AFI/SAFI for which it has received a nonzero
// Long-Lived Stale Time, the helper router MUST start a timer for that
// Long-Lived Stale Time."
// Each timer uses that family's original deadline.
func (m *grStateManager) enterLLGRLocked(peerAddr string, state *grPeerState, families []family.Family, deadline time.Time) *llgrPendingActions {
	if state.llgrFamilies == nil {
		state.llgrFamilies = make(map[family.Family]*time.Timer)
	}

	pending := &llgrPendingActions{peerAddr: peerAddr, mgr: m}

	// Build LLST lookup from LLGR capability
	llstByFamily := make(map[family.Family]uint32, len(state.llgrCap.Families))
	for _, f := range state.llgrCap.Families {
		if f.LLST > 0 {
			llstByFamily[f.Family] = f.LLST
		}
	}

	// Selection is bounded by the received capability family set.
	for _, fam := range families {
		if !state.staleFamilies[fam] {
			continue
		}
		if _, active := state.llgrFamilies[fam]; active {
			continue
		}
		llst, hasLLGR := llstByFamily[fam]
		if !hasLLGR {
			// Family not in LLGR cap or LLST=0 -> purge immediately
			pending.purged = append(pending.purged, fam)
			continue
		}

		// RFC 9494 Section 4.2: "The interval for which they are retained is limited
		// by the sum of the Restart Time in the received Graceful Restart Capability
		// and the Long-Lived Stale Time in the received Long-Lived Graceful Restart
		// Capability." Dispatch delay consumes this budget instead of extending it.
		remaining := time.Until(deadline.Add(time.Duration(llst) * time.Second))
		if remaining <= 0 {
			pending.purged = append(pending.purged, fam)
			continue
		}

		// Start per-family LLST timer with ownership guard.
		// Capture state pointer so stale callbacks from a previous GR cycle
		// can detect they no longer own the peer's state (consecutive restart).
		famCapture := fam // capture for closure
		owner := state
		timer := time.AfterFunc(remaining, func() {
			m.handleLLSTExpired(peerAddr, famCapture, owner)
		})
		state.llgrFamilies[fam] = timer

		// Collect callback action (fired after unlock)
		pending.entries = append(pending.entries, llgrEntryAction{family: fam, llst: llst})
	}

	// Remove purged families from stale tracking
	for _, fam := range pending.purged {
		delete(state.staleFamilies, fam)
	}

	state.inLLGR = len(state.llgrFamilies) > 0
	// A family still in conventional GR also keeps this peer's retained RIB.
	if len(state.staleFamilies) == 0 {
		m.deletePeerLocked(peerAddr)
		pending.complete = true
	}
	pending.entryDone = len(pending.entries) > 0

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
func (m *grStateManager) handleLLSTExpired(peerAddr string, fam family.Family, owner *grPeerState) {
	m.mu.Lock()

	state, ok := m.peers[peerAddr]
	if !ok || state != owner {
		m.mu.Unlock()
		return
	}

	delete(state.staleFamilies, fam)
	delete(state.llgrFamilies, fam)

	state.inLLGR = len(state.llgrFamilies) > 0
	lastFamily := len(state.staleFamilies) == 0
	if lastFamily {
		m.deletePeerLocked(peerAddr)
	}

	m.mu.Unlock()

	logger().Info("LLST timer expired", "peer", peerAddr, "family", fam, "last", lastFamily)

	if m.onLLGRFamilyExpired != nil {
		m.onLLGRFamilyExpired(peerAddr, fam)
	}
	if lastFamily && m.onLLGRComplete != nil {
		m.onLLGRComplete(peerAddr)
	}
}

// removePeer clears all GR/LLGR state and stops all timers for a peer that has
// been removed from configuration. Safe to call for a peer with no state.
func (m *grStateManager) removePeer(peerAddr string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.clearPeerLocked(peerAddr)
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
func (m *grStateManager) allStaleFamiliesLocked(state *grPeerState) []family.Family {
	families := make([]family.Family, 0, len(state.staleFamilies))
	for fam := range state.staleFamilies {
		families = append(families, fam)
	}
	return families
}
