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

// peerSettingsSwap is one reconcile decision: deliver next to the peer at key
// without restarting its session.
type peerSettingsSwap struct {
	key  netip.AddrPort
	next *PeerSettings
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
// Only two fields qualify today, and each one qualifies for a reason read at the
// consumer:
//
//   - ImportFilters: the ingress datapath re-reads it per received UPDATE through
//     the p.mu-guarded accessor Peer.ImportFilters (runIngressPolicyChain,
//     filter_ordered.go). Nothing caches it.
//   - ExportFilters: the egress datapath reads facts.exportFilters
//     (egress_inject_filter.go, forward_rs.go, reactor_api_forward.go), a snapshot
//     built by refreshForwardFacts. applyHotSwappableSettings rebuilds that
//     snapshot, which is what makes the field swappable rather than inert.
//
// Both are already the mutable pair: resolveDynamicPeerSettings (reactor_dynamic.go)
// writes them on the pointed-to struct under p.mu on a dynamic peer's establishment,
// and every reader outside the facts snapshot goes through the locked accessors.
// That is why writing them from the reload goroutine is race-free and why no other
// field is: the rest are read off p.settings and s.settings with no lock, under the
// immutability contract stated on Peer.Settings (peer.go).
func hotSwappableSettings(dst, src *PeerSettings) {
	dst.ImportFilters = src.ImportFilters
	dst.ExportFilters = src.ExportFilters
}

// peerSettingsRestartRequired reports whether applying next to a peer currently
// running current needs the session torn down and re-established.
func peerSettingsRestartRequired(current, next *PeerSettings) bool {
	return peerSettingsRestartReason(current, next) != ""
}

// peerSettingsRestartReason names the fields that force a restart, comma separated,
// or "" when the change is hot-swappable (or is no change at all).
//
// The reason is the decision, not a description of it: peerSettingsRestartRequired
// is defined as a non-empty reason, so the log line and the branch can never
// disagree. BIRD logs nothing for a successful soft reconfigure and this mirrors it:
// silence on a swap, one named line on a restart
// (docs/research/bird-bgp-reference.md).
func peerSettingsRestartReason(current, next *PeerSettings) string {
	// A missing side is not a comparison. Restart is the answer that cannot leave a
	// session running on settings nobody checked.
	if current == nil || next == nil {
		return "PeerSettings"
	}

	// Neutralize the hot-swappable fields by making current's copy hold next's
	// values. What remains different is what a swap cannot deliver.
	c := *current
	hotSwappableSettings(&c, next)
	if peerSettingsEqual(&c, next) {
		return ""
	}

	n := *next
	// Capabilities compare by wire encoding, matching peerSettingsEqual: two values
	// can differ in Go representation and encode identically.
	if !capabilitiesEqual(c.Capabilities, n.Capabilities) {
		return "Capabilities"
	}
	c.Capabilities, n.Capabilities = nil, nil
	// PrefixUpdated is the hidden PeeringDB staleness date (peersettings.go). It
	// drives no session or datapath behavior, and peerSettingsEqual ignores it.
	c.PrefixUpdated, n.PrefixUpdated = nil, nil

	cv, nv := reflect.ValueOf(c), reflect.ValueOf(n)
	structType := cv.Type()
	var changed []string
	for i := range structType.NumField() {
		if !reflect.DeepEqual(cv.Field(i).Interface(), nv.Field(i).Interface()) {
			changed = append(changed, structType.Field(i).Name)
		}
	}
	if len(changed) == 0 {
		// peerSettingsEqual said the two differ and no field walk found it. Naming
		// the struct keeps the branch honest rather than reporting "no restart" on
		// a difference nothing could account for.
		return "PeerSettings"
	}
	return strings.Join(changed, ",")
}

// swapPeerSettingsJournaled delivers each swap to its running peer, recording an
// undo that restores the peer's previous hot-swappable fields.
//
// The undo is a snapshot taken BEFORE the apply, because the apply writes onto the
// struct the reconcile loop holds as "current": reading the old values afterwards
// would read the new ones. A rollback therefore returns the running session to the
// chain it was enforcing, without restarting it either.
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
		previous := peer.hotSwappableSnapshot()
		err := j.Record(
			func() error {
				peer.applyHotSwappableSettings(next)
				return nil
			},
			func() error {
				peer.applyHotSwappableSettings(previous)
				return nil
			},
		)
		if err != nil {
			return fmt.Errorf("swap peer settings %s: %w", swap.key, err)
		}
	}

	return nil
}

// hotSwappableSnapshot copies the peer's current hot-swappable fields into a fresh
// PeerSettings. The reload journal keeps it as the undo value for a swap.
//
// Reading them through p.mu matters: resolveDynamicPeerSettings writes the same two
// fields on the establishment goroutine.
func (p *Peer) hotSwappableSnapshot() *PeerSettings {
	previous := &PeerSettings{}
	p.mu.RLock()
	hotSwappableSettings(previous, p.settings)
	p.mu.RUnlock()
	return previous
}

// applyHotSwappableSettings publishes next's hot-swappable fields onto a running
// peer. The session is not touched: the FSM, the TCP connection and the negotiated
// capabilities all survive.
//
// The write goes onto the pointed-to struct rather than replacing the pointer, and
// that is deliberate. Session holds its own copy of the same pointer, taken once in
// NewSession (peer_run.go), so replacing Peer.settings would leave every s.settings
// reader on the old struct — the silent-discard failure in a new place.
//
// refreshForwardFactsIfLive runs AFTER p.mu is released because it re-acquires
// p.mu.RLock and RWMutex is not reentrant. This mirrors resolveDynamicPeerSettings.
func (p *Peer) applyHotSwappableSettings(next *PeerSettings) {
	p.mu.Lock()
	hotSwappableSettings(p.settings, next)
	p.mu.Unlock()

	p.refreshForwardFactsIfLive()
}
