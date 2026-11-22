// Design: docs/architecture/core-design.md — graceful restart state management
// RFC: rfc/short/rfc4724.md — Receiving Speaker procedures (Section 4.2)
// RFC: rfc/short/rfc9494.md — Long-Lived Graceful Restart (LLGR) procedures
// Overview: gr.go — GR plugin entry point, event dispatch, and capability storage
// Related: gr_llgr.go — LLGR capability types used by state machine for LLST tracking

package gr

import (
	"cmp"
	"slices"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/core/family"
)

// grPeerCap holds the GR capability data extracted from a peer's OPEN message.
// This is a simplified representation of what we parse from the JSON event.
type grPeerCap struct {
	connectionID uint64 // immutable OPEN physical connection
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
	// source is the original retained obligation; it survives a reconnect.
	source grSource
	// reestablishedConnectionID is the successor allowed to complete EOR.
	// Zero means the original connection is down and no successor is up.
	reestablishedConnectionID uint64

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
	// Populated while at least one family has a running LLST timer, including
	// across session re-establishment until that family's EOR arrives (RFC 9494 §4.2).
	llgrFamilies map[family.Family]*time.Timer

	// llstExpiredDuringSync tracks families whose LLST timer expired after
	// session re-establishment while still awaiting EOR. If the session resets
	// again before EOR arrives for such a family, RFC 9494 Section 4.2 requires
	// any remaining routes for that family to be removed immediately rather
	// than retained for another GR/LLGR cycle.
	llstExpiredDuringSync map[family.Family]bool

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
	peers map[uint64]*grPeerState // original owner -> state

	// onTimerExpired is called when GR period ends without LLGR (purge all stale).
	onTimerExpired func(source grSource)

	// onLLGREnter is called per-family when transitioning from GR to LLGR.
	// The callback receives peer address, family, and LLST in seconds.
	onLLGREnter func(source grSource, fam family.Family, llst uint32)

	// onLLGRFamilyExpired is called when an LLST timer expires for one family.
	onLLGRFamilyExpired func(source grSource, fam family.Family)

	// onLLGREntryDone is called once after all families have entered LLGR.
	// Used to trigger per-family readvertisement of updated routes.
	// families contains the address families that entered LLGR (e.g., ipv4/unicast).
	onLLGREntryDone func(source grSource, families []family.Family)

	// onLLGRComplete is called when all LLGR families have expired or completed.
	onLLGRComplete func(source grSource)
}

// newGRStateManager creates a GR state manager.
// onExpired is called when a peer's restart timer fires and no LLGR is available.
func newGRStateManager(onExpired func(source grSource)) *grStateManager {
	return &grStateManager{
		peers:          make(map[uint64]*grPeerState),
		onTimerExpired: onExpired,
	}
}

// retainedFamilies answers the families whose routes are retained for
// source now, sorted so a verdict renders the same for the same state. No
// peer state answers none: the peer is in neither GR nor LLGR.
func (m *grStateManager) retainedFamilies(source grSource) []family.Family {
	m.mu.Lock()
	defer m.mu.Unlock()
	state := m.peers[source.owner]
	if state == nil {
		return nil
	}
	if state.source.connectionID != source.connectionID {
		return nil
	}
	retained := make([]family.Family, 0, len(state.staleFamilies))
	for fam, stale := range state.staleFamilies {
		if stale {
			retained = append(retained, fam)
		}
	}
	slices.SortFunc(retained, func(a, b family.Family) int { return cmp.Compare(a.String(), b.String()) })
	return retained
}

// retainedSource returns the original retained cycle for this actual owner.
// Safe for concurrent use. False means no retained cycle exists.
func (m *grStateManager) retainedSource(source grSource) (grSource, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	state := m.peers[source.owner]
	if state == nil {
		return grSource{}, false
	}
	return state.source, true
}

// llgrFamilyActive answers whether this original connection still owns a
// family's LLST timer. Safe for concurrent use. Callers MUST check after each
// blocking entry RPC before dispatching the next mutation.
// llgrFamilyActive answers whether this original connection still owns a
// pre-reconnect LLGR entry cycle for fam. Once a successor session has
// re-established (reestablishedConnectionID != 0), an in-flight LLGR entry
// tail from before reconnect must not continue mutating or readvertising
// routes on the re-established session.
func (m *grStateManager) llgrFamilyActive(source grSource, fam family.Family) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	state := m.peers[source.owner]
	if state == nil {
		return false
	}
	if state.source.connectionID != source.connectionID {
		return false
	}
	if state.reestablishedConnectionID != 0 {
		return false
	}
	_, active := state.llgrFamilies[fam]
	return active
}

// peerActive returns true if the peer is in GR or LLGR.
func (m *grStateManager) peerActive(source grSource) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.peers[source.owner]
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
func (m *grStateManager) onSessionDown(source grSource, cap *grPeerCap, llgrCap *llgrPeerCap, wasNotification bool) bool {
	activated, completeDown := m.onSessionDownDeferred(source, cap, llgrCap, wasNotification)
	if completeDown != nil {
		completeDown()
	}
	return activated
}

// sessionDownPlan describes the RIB actions a session-down transition requires.
type sessionDownPlan struct {
	activated         bool
	prevSource        grSource
	purgeAllPrevious  bool
	purgePreviousFams []family.Family
	// markStaleFams lists the families that start a conventional GR period or
	// enter LLGR at this DOWN while other families stay in their LLGR period;
	// each gets a per-family GR mark-stale. It is read only when
	// preservedLLGRActive is set, and is nil when this DOWN starts no such
	// family.
	markStaleFams []family.Family
	// preservedLLGRFams lists, sorted, the families whose LLST timer keeps
	// running across this DOWN. The routes the ended connection delivered for
	// them are retained too, so the caller marks them LLGR-stale. Nil unless
	// preservedLLGRActive is set.
	preservedLLGRFams   []family.Family
	preservedLLGRActive bool
	completeDown        func()
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
func (m *grStateManager) onSessionDownDeferred(source grSource, cap *grPeerCap, llgrCap *llgrPeerCap, wasNotification bool) (bool, func()) {
	plan := m.onSessionDownPlan(source, cap, llgrCap, wasNotification)
	return plan.activated, plan.completeDown
}

// onSessionDownPlan computes the session-down state transition while enforcing
// RFC 9494 Section 4.2 rules across consecutive restarts:
//   - Running LLST timers and their LLGR-stale routes are preserved untouched
//     (not purged, not reset or updated) until expiry or synchronization (EOR).
//     The state's source becomes the connection that just ended; the earlier
//     connections whose stale routes survive are the RIB's to name, and the
//     plugin addresses each one (grPlugin.dispatchRetained).
//   - Families whose LLST timer expired during an unsynchronized reconnect are
//     excluded from retention and purged immediately on reset.
//   - Conventional GR-only stale families from a prior cycle are purged per
//     RFC 4724 Section 4.2.
func (m *grStateManager) onSessionDownPlan(source grSource, cap *grPeerCap, llgrCap *llgrPeerCap, wasNotification bool) sessionDownPlan {
	m.mu.Lock()

	prev := m.peers[source.owner]
	var plan sessionDownPlan
	if prev != nil {
		plan.prevSource = prev.source
	}

	// No GR capability or NOTIFICATION -> standard BGP (no route retention)
	if cap == nil || wasNotification {
		if prev != nil {
			plan.purgeAllPrevious = true
		}
		m.clearPeerLocked(source.grPeer)
		m.mu.Unlock()
		return plan
	}

	// Partition any prior retained cycle: running LLST timers survive a
	// consecutive restart untouched (RFC 9494 §4.2), expired-during-sync
	// families must be purged immediately and excluded from new retention,
	// and conventional GR families are purged per RFC 4724 §4.2.
	var preservedLLGR map[family.Family]*time.Timer
	var expiredDuringSync map[family.Family]bool
	if prev != nil {
		if prev.restartTimer != nil {
			prev.restartTimer.Stop()
			prev.restartTimer = nil
		}
		prev.restartDeadline = time.Time{}

		if len(prev.llgrFamilies) > 0 {
			preservedLLGR = prev.llgrFamilies
			plan.preservedLLGRActive = true
		}
		if len(prev.llstExpiredDuringSync) > 0 {
			expiredDuringSync = prev.llstExpiredDuringSync
		}

		if len(preservedLLGR) == 0 && len(expiredDuringSync) == 0 {
			// Plain RFC 4724 consecutive restart: purge previous cycle cleanly.
			plan.purgeAllPrevious = true
			delete(m.peers, source.owner)
			prev = nil
		} else {
			for fam := range prev.staleFamilies {
				if _, kept := preservedLLGR[fam]; !kept {
					plan.purgePreviousFams = append(plan.purgePreviousFams, fam)
				}
			}
			for fam := range expiredDuringSync {
				plan.purgePreviousFams = append(plan.purgePreviousFams, fam)
			}
		}
	}

	// RFC 9494 Section 4.2: "If the Graceful Restart Capability that was received
	// does not list all AFIs/SAFIs supported by the session, then the GR Restart
	// Time shall be deemed zero for those AFIs/SAFIs that are not listed."
	downAt := time.Now()
	staleFamilies := make(map[family.Family]bool, len(cap.Families)+len(preservedLLGR))
	for fam := range preservedLLGR {
		staleFamilies[fam] = true
	}

	var newGRFamilies []family.Family
	for _, f := range cap.Families {
		if expiredDuringSync[f.Family] {
			continue
		}
		if _, kept := preservedLLGR[f.Family]; kept {
			continue
		}
		staleFamilies[f.Family] = true
		newGRFamilies = append(newGRFamilies, f.Family)
	}

	var immediate []family.Family
	if llgrCap != nil {
		for _, f := range llgrCap.Families {
			if expiredDuringSync[f.Family] {
				continue
			}
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
		delete(m.peers, source.owner)
		m.mu.Unlock()
		return plan
	}

	var state *grPeerState
	if prev != nil && len(preservedLLGR) > 0 {
		// Reuse prev in place so running LLST timer closures (which guard on
		// owner == state) remain authoritative for the preserved families.
		state = prev
		state.source = source
		state.reestablishedConnectionID = 0
		state.staleFamilies = staleFamilies
		state.llgrFamilies = preservedLLGR
		state.llstExpiredDuringSync = nil
		state.inLLGR = true
		state.llgrCap = llgrCap
		if len(newGRFamilies) > 0 {
			state.restartDeadline = downAt.Add(time.Duration(cap.RestartTime) * time.Second)
		} else {
			state.restartDeadline = time.Time{}
		}
		plan.markStaleFams = append(newGRFamilies, immediate...)
		plan.preservedLLGRFams = make([]family.Family, 0, len(preservedLLGR))
		for fam := range preservedLLGR {
			plan.preservedLLGRFams = append(plan.preservedLLGRFams, fam)
		}
		slices.SortFunc(plan.preservedLLGRFams, func(a, b family.Family) int { return cmp.Compare(a.String(), b.String()) })
	} else {
		state = &grPeerState{
			source:        source,
			staleFamilies: staleFamilies,
			llgrCap:       llgrCap,
		}
		if len(newGRFamilies) > 0 {
			state.restartDeadline = downAt.Add(time.Duration(cap.RestartTime) * time.Second)
		}
		m.peers[source.owner] = state
	}

	m.mu.Unlock()
	plan.activated = true
	plan.completeDown = func() {
		var pending *llgrPendingActions
		if len(immediate) > 0 {
			m.mu.Lock()
			if m.peers[source.owner] != state {
				m.mu.Unlock()
				return
			}
			// A reconnect retains this same state pointer, but it has revoked
			// the old DOWN's deferred immediate-entry completion.
			if state.reestablishedConnectionID != 0 {
				m.mu.Unlock()
				return
			}
			// RFC 9494 Section 4.2. Retention and stale marking have completed;
			// omitted families start their LLST budget at the original DOWN.
			pending = m.enterLLGRLocked(source, state, immediate, downAt)
			m.mu.Unlock()
		}
		// RFC 9494 Section 4.2: "The interval for which they are retained is limited
		// by the sum of the Restart Time in the received Graceful Restart Capability
		// and the Long-Lived Stale Time in the received Long-Lived Graceful Restart
		// Capability." An immediate family's RPC MUST NOT delay a sibling's timer.
		m.startRestartTimer(source, state)
		if pending != nil {
			pending.fire()
		}
	}
	return plan
}

// startRestartTimer completes session-down only after the RIB has marked routes
// stale. Safe for concurrent use; callers MUST NOT hold m.mu.
//
// RFC 4724 Section 4.2: "If the session does not get re-established within the
// "Restart Time" that the peer advertised previously, the Receiving Speaker MUST
// delete all the stale routes from the peer that it is retaining."
// The deadline remains anchored to the original session-down time.
func (m *grStateManager) startRestartTimer(source grSource, state *grPeerState) {
	m.mu.Lock()
	if m.peers[source.owner] != state {
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
			m.handleTimerExpired(source, state)
		})
		m.mu.Unlock()
		return
	}
	m.mu.Unlock()

	// RFC 4724 Section 4.2. A zero or elapsed deadline expires synchronously,
	// after stale marking, rather than racing the session-down commands.
	m.handleTimerExpired(source, state)
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
// RFC 9494 Section 4.2: a family whose LLGR period has begun (it holds an LLST
// timer) must ALSO pass the new LLGR capability: F bit clear, or the family
// missing from it, purges the family. A family still in its conventional GR
// period is judged by the RFC 4724 rule alone. The two rules combine with AND
// (owner decision Q-1, 2026-10-14, spec-bgp-graceful-restart-rfc-defects),
// because the same section says the RFC 4724 procedures apply "irrespective of
// whether the stale routes are retained during the Restart Time period or the
// Long-Lived Stale Time period".
func (m *grStateManager) onSessionReestablished(source grSource, newCap *grPeerCap, newLLGRCap *llgrPeerCap) (grSource, []family.Family, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, ok := m.peers[source.owner]
	if !ok {
		return grSource{}, nil, false // No retained obligation.
	}

	// The original physical connection cannot reestablish itself after DOWN.
	if source.connectionID == state.source.connectionID {
		return grSource{}, nil, false
	}
	retainedSource := state.source
	state.reestablishedConnectionID = source.connectionID
	wasInLLGR := state.inLLGR
	// The per-family "LLGR period has begun" set, captured before
	// stopLLSTTimersLocked clears it. inLLGR is peer-wide and cannot say which
	// family is in which period.
	inLLGRPeriod := make(map[family.Family]bool, len(state.llgrFamilies))
	for fam := range state.llgrFamilies {
		inLLGRPeriod[fam] = true
	}

	// Cancel callbacks that already started and cannot be stopped by Timer.Stop.
	state.restartDeadline = time.Time{}
	// Stop GR restart timer if still running.
	if state.restartTimer != nil {
		state.restartTimer.Stop()
		state.restartTimer = nil
	}

	// No GR cap in new OPEN -> stop all LLST timers and purge all
	if newCap == nil {
		m.stopLLSTTimersLocked(state)
		purged := m.allStaleFamiliesLocked(state)
		m.deletePeerLocked(source.grPeer)
		return retainedSource, purged, wasInLLGR
	}

	// RFC 4724 Section 4.2: the families the new GR capability lists with the
	// "Forwarding State" bit set.
	grForwarding := make(map[family.Family]bool, len(newCap.Families))
	for _, f := range newCap.Families {
		if f.ForwardState {
			grForwarding[f.Family] = true
		}
	}
	// RFC 9494 Section 4.2: the families the new LLGR capability lists with the
	// F bit set. A nil capability lists none, so every LLGR-period family then
	// fails the "not included" bullet.
	llgrForwarding := make(map[family.Family]bool)
	if newLLGRCap != nil {
		for _, f := range newLLGRCap.Families {
			if f.ForwardState {
				llgrForwarding[f.Family] = true
			}
		}
	}

	// RFC 9494 Section 4.2: "However, the timer continues to run once the
	// session has re-established. The timer is neither stopped nor updated
	// until the EoR marker is received for the relevant AFI/SAFI from the
	// peer." Only families purged immediately on reconnect have their LLST
	// timer stopped here.
	var purged []family.Family
	for fam := range state.staleFamilies {
		// RFC 4724 Section 4.2, RFC 9494 Section 4.2.
		if reconnectKeepsFamily(fam, grForwarding, llgrForwarding, inLLGRPeriod) {
			continue
		}
		if timer, active := state.llgrFamilies[fam]; active {
			timer.Stop()
			delete(state.llgrFamilies, fam)
		}
		purged = append(purged, fam)
		delete(state.staleFamilies, fam)
	}

	// Update LLGR cap for potential future LLGR cycle; preserve inLLGR while
	// any surviving family's LLST timer is still running.
	state.llgrCap = newLLGRCap
	state.inLLGR = len(state.llgrFamilies) > 0

	// If all families purged and none expired during sync, GR/LLGR is complete.
	if len(state.staleFamilies) == 0 && len(state.llstExpiredDuringSync) == 0 {
		m.deletePeerLocked(source.grPeer)
	}

	return retainedSource, purged, wasInLLGR
}

// reconnectKeepsFamily answers whether a stale family survives the re-established
// session's capabilities. It is pure: the caller holds m.mu and owns the maps.
func reconnectKeepsFamily(fam family.Family, grForwarding, llgrForwarding, inLLGRPeriod map[family.Family]bool) bool {
	// RFC 4724 Section 4.2: "if the "Forwarding State" bit for a specific address
	// family is not set in the newly received Graceful Restart Capability, or if a
	// specific address family is not included in the newly received Graceful
	// Restart Capability, ... the Receiving Speaker MUST immediately remove all
	// the stale routes from the peer that it is retaining for that address family."
	if !grForwarding[fam] {
		return false
	}
	if !inLLGRPeriod[fam] {
		return true
	}
	// RFC 9494 Section 4.2: "once the LLGR Period begins, the Helper MUST
	// immediately remove all the stale routes from the peer that it is retaining
	// for that address family if any of the following occur: * the F bit for a
	// specific address family is not set in the newly received LLGR Capability,
	// or * a specific address family is not included in the newly received LLGR
	// Capability". A clear F bit and a missing family both leave fam out of
	// llgrForwarding.
	return llgrForwarding[fam]
}

// onEORReceived is called when End-of-RIB is received from a GR/LLGR-active peer.
// Returns true if stale routes for this family should be purged.
//
// RFC 4724 Section 4.2: On EOR receipt, immediately remove stale routes for that family.
// RFC 9494: During LLGR, also stop the LLST timer for that family.
func (m *grStateManager) onEORReceived(source grSource, fam family.Family) (grSource, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, ok := m.peers[source.owner]
	if !ok {
		return grSource{}, false
	}

	// refuse-by-default: guards stale purge -- only the successor's EOR
	// can complete the retained original physical source obligation.
	if state.reestablishedConnectionID != source.connectionID {
		return grSource{}, false
	}
	retainedSource := state.source
	wasStale := state.staleFamilies[fam]
	wasExpiredDuringSync := state.llstExpiredDuringSync[fam]
	if !wasStale && !wasExpiredDuringSync {
		return grSource{}, false
	}

	delete(state.staleFamilies, fam)
	delete(state.llstExpiredDuringSync, fam)

	// Stop LLST timer for this family if still running (RFC 9494 Section 4.2:
	// timer runs until EoR is received for the relevant AFI/SAFI).
	if timer, active := state.llgrFamilies[fam]; active {
		timer.Stop()
		delete(state.llgrFamilies, fam)
		state.inLLGR = len(state.llgrFamilies) > 0
	}

	// All families synchronized -> GR/LLGR complete
	if len(state.staleFamilies) == 0 && len(state.llstExpiredDuringSync) == 0 {
		if state.restartTimer != nil {
			state.restartTimer.Stop()
		}
		m.stopLLSTTimersLocked(state)
		m.deletePeerLocked(source.grPeer)
	}

	return retainedSource, wasStale
}

// llgrPendingActions collects callbacks to fire after releasing the state manager lock.
// Prevents holding m.mu across blocking RPCs (dispatchCommand).
type llgrPendingActions struct {
	entries   []llgrEntryAction // onLLGREnter per family
	purged    []family.Family   // onLLGRFamilyExpired per family
	complete  bool              // onLLGRComplete (all families done immediately)
	entryDone bool              // onLLGREntryDone (readvertisement trigger)
	source    grSource
	state     *grPeerState
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
		p.mgr.mu.Lock()
		current := p.mgr.peers[p.source.owner]
		sameCycle := current == p.state
		p.mgr.mu.Unlock()
		if !sameCycle {
			continue
		}
		if !p.mgr.llgrFamilyActive(p.source, e.family) {
			continue
		}
		if p.mgr.onLLGREnter != nil {
			p.mgr.onLLGREnter(p.source, e.family, e.llst)
		}
	}
	for _, fam := range p.purged {
		if p.mgr.onLLGRFamilyExpired != nil {
			p.mgr.onLLGRFamilyExpired(p.source, fam)
		}
	}
	if p.complete && p.mgr.onLLGRComplete != nil {
		p.mgr.onLLGRComplete(p.source)
	}
	if p.entryDone && p.mgr.onLLGREntryDone != nil {
		families := make([]family.Family, 0, len(p.entries))
		for _, e := range p.entries {
			if p.mgr.llgrFamilyActive(p.source, e.family) {
				families = append(families, e.family)
			}
		}
		if len(families) > 0 {
			p.mgr.onLLGREntryDone(p.source, families)
		}
	}
}

// handleTimerExpired handles GR restart timer expiry for a peer.
// RFC 4724 Section 4.2: delete all stale routes from the peer.
// RFC 9494: If LLGR negotiated, transition to LLGR instead of purging.
// The owner must be the cycle that armed the callback: Timer.Stop cannot cancel
// a callback that already started and is waiting for m.mu.
func (m *grStateManager) handleTimerExpired(source grSource, owner *grPeerState) {
	m.mu.Lock()

	state, ok := m.peers[source.owner]
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
		pending := m.enterLLGRLocked(source, state, m.allStaleFamiliesLocked(state), state.restartDeadline)
		state.restartDeadline = time.Time{}
		state.restartTimer = nil
		m.mu.Unlock()
		pending.fire()
		return
	}

	// No LLGR: standard GR expiry -- purge all stale
	m.clearPeerLocked(source.grPeer)
	m.mu.Unlock()

	logger().Info("GR restart timer expired, purging stale routes", "peer", source)
	if m.onTimerExpired != nil {
		m.onTimerExpired(source)
	}
}

// enterLLGRLocked transitions the selected families whose GR period has ended.
// Caller MUST hold m.mu and MUST fire the returned callbacks after unlocking.
// Existing family timers are preserved; deadline is the selected GR period's end.
// RFC 9494 Section 4.2: "For each AFI/SAFI for which it has received a nonzero
// Long-Lived Stale Time, the helper router MUST start a timer for that
// Long-Lived Stale Time."
// Each timer uses that family's original deadline.
func (m *grStateManager) enterLLGRLocked(source grSource, state *grPeerState, families []family.Family, deadline time.Time) *llgrPendingActions {
	if state.llgrFamilies == nil {
		state.llgrFamilies = make(map[family.Family]*time.Timer)
	}

	pending := &llgrPendingActions{source: source, state: state, mgr: m}

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
			m.handleLLSTExpired(source, famCapture, owner)
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
		m.deletePeerLocked(source.grPeer)
		pending.complete = true
	}
	pending.entryDone = len(pending.entries) > 0

	logger().Info("entered LLGR period",
		"peer", source,
		"llgr-families", len(state.llgrFamilies),
		"purged-families", len(pending.purged))

	return pending
}

// handleLLSTExpired handles LLST timer expiry for a specific family.
// RFC 9494: Delete stale routes for that family. If last family, release all.
// The owner parameter guards against stale callbacks from a previous GR cycle:
// if a consecutive restart replaced the peer's state, this callback is a no-op.
func (m *grStateManager) handleLLSTExpired(source grSource, fam family.Family, owner *grPeerState) {
	m.mu.Lock()

	state, ok := m.peers[source.owner]
	if !ok || state != owner {
		m.mu.Unlock()
		return
	}

	// Timer.Stop cannot retract a callback already waiting for mu. A
	// reconnect cleared this exact timer, so that callback no longer owns
	// even a retained state's family. A newer cycle also fails pointer identity.
	if _, active := state.llgrFamilies[fam]; !active {
		m.mu.Unlock()
		return
	}
	expiredSource := state.source
	delete(state.staleFamilies, fam)
	delete(state.llgrFamilies, fam)

	syncing := state.reestablishedConnectionID != 0
	if syncing {
		// RFC 9494 Section 4.2: "If the timer expires during synchronization
		// with the peer, any stale routes that the peer has not refreshed are
		// removed. If the session subsequently resets prior to becoming
		// synchronized, any remaining routes (for the AFI/SAFI whose LLST timer
		// expired) MUST be removed immediately."
		if state.llstExpiredDuringSync == nil {
			state.llstExpiredDuringSync = make(map[family.Family]bool)
		}
		state.llstExpiredDuringSync[fam] = true
	}

	state.inLLGR = len(state.llgrFamilies) > 0
	lastFamily := !syncing && len(state.staleFamilies) == 0 && len(state.llstExpiredDuringSync) == 0
	if lastFamily {
		m.deletePeerLocked(source.grPeer)
	}

	m.mu.Unlock()

	logger().Info("LLST timer expired", "peer", expiredSource, "family", fam, "last", lastFamily, "syncing", syncing)

	if m.onLLGRFamilyExpired != nil {
		m.onLLGRFamilyExpired(expiredSource, fam)
	}
	if lastFamily && m.onLLGRComplete != nil {
		m.onLLGRComplete(expiredSource)
	}
}

// removePeer clears all GR/LLGR state and stops all timers for a peer that has
// been removed from configuration. Safe to call for a peer with no state.
func (m *grStateManager) removePeer(source grPeer) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.clearPeerLocked(source)
}

// clearPeerLocked removes GR/LLGR state for a peer, stopping all active timers.
// Must be called with m.mu held.
func (m *grStateManager) clearPeerLocked(source grPeer) {
	if state, ok := m.peers[source.owner]; ok {
		if state.restartTimer != nil {
			state.restartTimer.Stop()
		}
		m.stopLLSTTimersLocked(state)
		delete(m.peers, source.owner)
	}
}

// deletePeerLocked removes the peer from the map without stopping timers.
// Used when timers have already been stopped individually.
// Must be called with m.mu held.
func (m *grStateManager) deletePeerLocked(source grPeer) {
	delete(m.peers, source.owner)
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
