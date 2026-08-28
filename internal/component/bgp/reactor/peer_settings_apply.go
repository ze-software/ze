// Design: docs/architecture/core-design.md — config reload delivers changed peer settings
// Related: reactor_api.go — reconcilePeersJournaled picks swap or restart with these
// Related: reactor_dynamic.go — resolveDynamicPeerSettings, the same guarded write
// Related: peer_forward_facts.go — the egress snapshot a swap must rebuild
package reactor

import (
	"fmt"
	"net/netip"
	"reflect"
	"strings"
)

// settingsCopier copies from src to dst the PeerSettings fields ONE swap decision
// classified as deliverable to a running session.
//
// It travels with the decision rather than being looked up at the apply site
// because the swap set is not fixed: a capability change joins it only when the
// negotiation was proved unchanged (peer_settings_negotiation.go). Passing the
// function keeps the invariant the original defect lacked -- the set neutralized
// when deciding IS the set applied afterwards, so a field cannot be judged
// swappable and then not delivered.
type settingsCopier func(dst, src *PeerSettings)

// peerSettingsSwap is one reconcile decision: deliver next to the peer at key
// without restarting its session.
type peerSettingsSwap struct {
	key   netip.AddrPort
	next  *PeerSettings
	apply settingsCopier
}

// hotSwappableSettings copies from src to dst every PeerSettings field a RUNNING
// session can take a new value for.
//
// It is the ONE definition of the "swap in place" category. peerSettingsRestartReason
// neutralizes exactly the fields this function copies, and applyHotSwappableSettings
// applies exactly those fields, so a field cannot be classified swappable without
// also being delivered. That is the property the original defect lacked: a field
// judged "no change" was never delivered, and the operator's edit was discarded.
//
// FAIL-CLOSED (ai/rules/fail-closed-guards.md): the list here is a SUBTRACTION from
// the whole struct, never an enumeration of what matters. Every field this function
// does not copy forces a session restart, so a field added to PeerSettings tomorrow
// is restart-scoped until somebody classifies it on purpose. A wrongly restarted
// session is visible and self-healing; a session left running on stale settings is
// the silent mis-enforcement this spec exists to remove.
//
// Three fields qualify today, and each one qualifies for a reason read at the
// consumer:
//
//   - ImportFilters: the ingress datapath re-reads it per received UPDATE through
//     the p.mu-guarded accessor Peer.ImportFilters (runIngressPolicyChain,
//     filter_ordered.go). Nothing caches it.
//   - ExportFilters: the egress datapath reads facts.exportFilters
//     (egress_inject_filter.go, forward_rs.go, reactor_api_forward.go), a snapshot
//     built by refreshForwardFacts. applyHotSwappableSettings rebuilds that
//     snapshot, which is what makes the field swappable rather than inert.
//   - PrefixUpdated: the per-family PeeringDB refresh dates. They feed no session
//     and no datapath: their two consumers are the prefix-stale report bus warning
//     and the ze_bgp_prefix_stale gauge, both published from the dates alone
//     (raisePrefixStale and setPrefixStaleMetric, session_prefix.go).
//     applyHotSwappableSettings republishes both, which is what makes this field
//     swappable rather than inert. Every reader outside AddPeer goes through the
//     p.mu-guarded accessor Peer.oldestPrefixUpdated (peer.go).
//
// All three are the mutable set: resolveDynamicPeerSettings (reactor_dynamic.go)
// writes the filter pair on the pointed-to struct under p.mu on a dynamic peer's
// establishment, and every reader outside the facts snapshot goes through the
// locked accessors. That is why writing them from the reload goroutine is race-free
// and why no other field is: the rest are read off p.settings and s.settings with
// no lock, under the immutability contract stated on Peer.Settings (peer.go).
func hotSwappableSettings(dst, src *PeerSettings) {
	dst.ImportFilters = src.ImportFilters
	dst.ExportFilters = src.ExportFilters
	dst.PrefixUpdated = src.PrefixUpdated
}

// peerSettingsRestartRequired reports whether applying next to a peer currently
// running current on session s needs the session torn down and re-established.
func peerSettingsRestartRequired(current, next *PeerSettings, s *Session) bool {
	return peerSettingsRestartReason(current, next, s) != ""
}

// peerSettingsRestartReason names the fields that force a restart, comma separated,
// or "" when the change is swappable (or is no change at all).
//
// The reason is the decision, not a description of it: peerSettingsRestartRequired
// is defined as a non-empty reason, so the log line and the branch can never
// disagree. BIRD logs nothing for a successful soft reconfigure and this mirrors it:
// silence on a swap, one named line on a restart
// (docs/research/bird-bgp-reference.md).
func peerSettingsRestartReason(current, next *PeerSettings, s *Session) string {
	_, reason := peerSettingsSwapPlan(current, next, s)
	return reason
}

// peerSettingsSwapPlan answers swap-or-restart for one peer, and on a swap also
// returns the copier that delivers exactly the fields it judged deliverable.
//
// Two categories can be delivered, and they qualify for DIFFERENT reasons. Keeping
// them apart is what stops the second from over-reaching:
//
//   - Always swappable: the fields a running session re-reads or republishes
//     (hotSwappableSettings). They have nothing to do with negotiation.
//   - Swappable when the negotiation is proved unchanged: the capability set, per
//     the owner's ruling of 2026-08-07 (negotiationOutcomeUnchanged,
//     peer_settings_negotiation.go). This one is conditional on evidence from the
//     RUNNING session, so it is asked only when the capabilities actually differ,
//     and it answers false whenever it cannot prove the outcome identical.
//
// Everything else restarts, including a field added to PeerSettings tomorrow: the
// decision is !peerSettingsEqual over the whole struct, minus what a copier
// neutralized (ai/rules/evidence.md, AC-6).
func peerSettingsSwapPlan(current, next *PeerSettings, s *Session) (settingsCopier, string) {
	// A missing side is not a comparison. Restart is the answer that cannot leave a
	// session running on settings nobody checked.
	if current == nil || next == nil {
		return nil, "PeerSettings"
	}

	// Neutralize the hot-swappable fields by making current's copy hold next's
	// values. What remains different is what a swap cannot deliver.
	c := *current
	hotSwappableSettings(&c, next)
	if peerSettingsEqual(&c, next) {
		return hotSwappableSettings, ""
	}

	// The ruling: a capability set that negotiates to the same encoding and the
	// same families against THIS peer's advertised capabilities leaves the running
	// session already correct, so deliver it and keep the session up.
	if !capabilitiesEqual(current.Capabilities, next.Capabilities) && s.negotiationOutcomeUnchanged(next) {
		negotiatedCapabilitySettings(&c, next)
		if peerSettingsEqual(&c, next) {
			return hotSwappableWithCapabilities, ""
		}
	}

	n := *next
	var changed []string
	// Capabilities compare by wire encoding, matching peerSettingsEqual: two values
	// can differ in Go representation and encode identically. Naming it does not end
	// the walk: a reload that rotates the router id AND edits the capability block
	// forced the restart for two reasons, and an operator reading one of them would
	// fix one and see the session flap again.
	if !capabilitiesEqual(c.Capabilities, n.Capabilities) {
		changed = append(changed, "Capabilities")
	}
	c.Capabilities, n.Capabilities = nil, nil

	cv, nv := reflect.ValueOf(c), reflect.ValueOf(n)
	structType := cv.Type()
	for i := range structType.NumField() {
		if !reflect.DeepEqual(cv.Field(i).Interface(), nv.Field(i).Interface()) {
			changed = append(changed, structType.Field(i).Name)
		}
	}
	if len(changed) == 0 {
		// peerSettingsEqual said the two differ and no field walk found it. Naming
		// the struct keeps the branch honest rather than reporting "no restart" on
		// a difference nothing could account for.
		return nil, "PeerSettings"
	}
	return nil, strings.Join(changed, ",")
}

// swapPeerSettingsJournaled delivers each swap to its running peer, recording an
// undo that restores the peer's previous hot-swappable fields.
//
// The undo is read from the RUNNING peer, and BEFORE the apply. Each half has its
// own reason. The reconcile loop's "current" is a snapshot taken at the top of the
// diff (settingsSnapshot, reactor_api.go), so it is not the peer's live value by
// the time a swap runs. The apply then writes onto the struct the peer holds, so
// reading it afterwards would read the new values. A rollback therefore returns
// the running session to the chain it was enforcing, and restarts nothing either.
//
// A peer that left the map between the diff and the apply is skipped rather than
// treated as an error: it is already gone, so there is nothing to keep current.
func (a *reactorAPIAdapter) swapPeerSettingsJournaled(swaps []peerSettingsSwap, j configJournal) error {
	r := a.r

	for _, swap := range swaps {
		r.mu.RLock()
		peer := r.peers[swap.key]
		r.mu.RUnlock()
		if peer == nil {
			continue
		}

		next := swap.next
		apply := swap.apply
		previous := peer.hotSwappableSnapshot(apply)
		err := j.Record(
			func() error {
				peer.applyHotSwappableSettings(next, apply)
				return nil
			},
			func() error {
				peer.applyHotSwappableSettings(previous, apply)
				return nil
			},
		)
		if err != nil {
			return fmt.Errorf("swap peer settings %s: %w", swap.key, err)
		}
	}

	return nil
}

// hotSwappableSnapshot copies the peer's current values of the fields copy
// delivers into a fresh PeerSettings. The reload journal keeps it as the undo
// value for a swap, so the snapshot must cover exactly the same fields the apply
// will overwrite: a narrower snapshot leaves a rollback that restores some of them.
//
// Reading them through p.mu matters: resolveDynamicPeerSettings writes two of the
// same fields on the establishment goroutine.
func (p *Peer) hotSwappableSnapshot(copy settingsCopier) *PeerSettings {
	previous := &PeerSettings{}
	p.mu.RLock()
	copy(previous, p.settings)
	p.mu.RUnlock()
	return previous
}

// applyHotSwappableSettings publishes the fields copy delivers from next onto a
// running peer. The session is not touched: the FSM, the TCP connection and the
// negotiated capabilities all survive.
//
// copy is the SAME function the decision neutralized with (peerSettingsSwapPlan),
// so the peer receives exactly what the decision judged deliverable, and no field
// can be classified swappable without being applied.
//
// The write goes onto the pointed-to struct rather than replacing the pointer, and
// that is deliberate. Session holds its own copy of the same pointer, taken once in
// NewSession (peer_run.go), so replacing Peer.settings would leave every s.settings
// reader on the old struct — the silent-discard failure in a new place.
//
// A new capability set needs no republication, and that is a property of its
// consumers rather than an omission: buildOpen reads it once per connection and
// every other reader reads it on demand, both through ConfiguredCapabilities
// (peer_settings_negotiation.go). The running session keeps the negotiation the
// decision proved unchanged, and the new set governs the next OPEN.
//
// refreshForwardFactsIfLive and refreshPrefixStale run AFTER p.mu is released
// because both re-acquire p.mu.RLock and RWMutex is not reentrant. This mirrors
// resolveDynamicPeerSettings.
func (p *Peer) applyHotSwappableSettings(next *PeerSettings, copy settingsCopier) {
	p.mu.Lock()
	copy(p.settings, next)
	p.mu.Unlock()

	p.refreshForwardFactsIfLive()
	p.refreshPrefixStale()
}

// refreshPrefixStale republishes the prefix-staleness verdict from the peer's
// CURRENT PrefixUpdated dates, on the two surfaces Reactor.AddPeer publishes it on:
// the prefix-stale report bus warning (`ze show warnings`, the login banner) and
// the ze_bgp_prefix_stale gauge.
//
// Both directions matter and both are covered, because raisePrefixStale and
// setPrefixStaleMetric each derive the verdict from the date rather than only
// raising on a bad one (session_prefix.go): a PeeringDB refresh that makes the
// dates fresh CLEARS the warning and sets the gauge to 0, and a config that lets
// them go past the 180-day threshold raises it.
//
// Delivering the dates without this call would move the silent-discard failure one
// layer out: the running peer would hold the new dates and both alarms would still
// answer from the ones read at AddPeer, so the alarm a refresh exists to clear
// could only clear on a daemon restart
// (plan/deferrals/fixit-bgp-per-family-prefix-enforcement.md).
func (p *Peer) refreshPrefixStale() {
	p.mu.RLock()
	oldest := p.settings.OldestPrefixUpdated()
	p.mu.RUnlock()

	now := p.clock.Now()
	raisePrefixStale(p.addrString, oldest, now)
	if p.reactor != nil {
		setPrefixStaleMetric(p.reactor.rmetrics, p.addrString, oldest, now)
	}
}
