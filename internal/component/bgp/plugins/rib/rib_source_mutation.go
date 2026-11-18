// Design: docs/guide/graceful-restart.md -- bounded original-physical GR mutation.
// Related: rib_source_commands.go -- parsed physical scope and registrations.
package rib

import (
	"context"
	"errors"
	"net/netip"

	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/opererr"
)

func (source ribPhysicalSource) matchesReceived(entry *storage.RouteEntry) bool {
	if entry.SourcePeerOwner() != source.owner {
		return false
	}
	return entry.SourceConnectionID() == source.connection
}

func (source ribPhysicalSource) matchesSent(entry *ribOutEntry) bool {
	if entry.SourceOwner != source.owner {
		return false
	}
	return entry.SourceConnectionID == source.connection
}

func (source ribPhysicalSource) ownsState(state *peerGRState) bool {
	if state == nil {
		return false
	}
	if state.source == nil {
		return false
	}
	return *state.source == source
}

// physicalSourceKnownLocked recognizes only live metadata, the existing active
// GR cycle, or an actually held received reference. It stores no completed
// source identities. Caller MUST hold peerMu.
func (r *RIBManager) physicalSourceKnownLocked(source ribPhysicalSource) bool {
	if source.ownsState(r.grState[source.owner]) {
		return true
	}
	if metadata := r.peerMeta[source.owner]; metadata != nil {
		if metadata.ConnectionID == source.connection {
			return true
		}
	}
	return r.physicalSourceHasRoutesLocked(source)
}

func (r *RIBManager) physicalSourceHasRoutesLocked(source ribPhysicalSource) bool {
	routes := r.bgpPeers[source.owner]
	if routes == nil {
		return false
	}
	for _, fam := range routes.Families() {
		found := false
		routes.IterateFamily(fam, func(_ []byte, entry storage.RouteEntry) bool {
			found = source.matchesReceived(&entry)
			return !found
		})
		if found {
			return true
		}
	}
	return false
}

// retainPhysicalStateLocked creates only a current retained cycle. An old
// connection cannot replace a successor's prepared state. A new current
// connection replaces an empty old cycle; unresolved output stays on its
// original existing lease rather than in GR history. Caller MUST hold peerMu.
func (r *RIBManager) retainPhysicalStateLocked(source ribPhysicalSource) (*peerGRState, error) {
	if r.stopping {
		return nil, sourceCommandRefused("The RIB is stopping")
	}
	previous := r.grState[source.owner]
	if source.ownsState(previous) {
		return previous, nil
	}
	metadata := r.peerMeta[source.owner]
	if metadata == nil {
		return nil, sourceCommandRefused("The original peer has no current received metadata")
	}
	if metadata.ConnectionID != source.connection {
		return nil, sourceCommandRefused("The retention command does not name the current original connection")
	}
	if previous != nil {
		if previous.source != nil {
			if r.physicalSourceHasRoutesLocked(*previous.source) {
				return nil, sourceCommandRefused("A previous retained physical connection still owns routes")
			}
		}
		r.stopGRExpiryLocked(previous)
	}
	current := &peerGRState{source: &source}
	r.grState[source.owner] = current
	return current, nil
}

// physicalSourceFamilies returns the existing finite family inventory when
// no family was requested. A non-nil selection was validated at command input.
func physicalSourceFamilies(routes *storage.PeerRIB, selected *family.Family) []family.Family {
	if selected != nil {
		return []family.Family{*selected}
	}
	return routes.Families()
}

type physicalSourceRemoval struct {
	count   int
	removed removedReceivedOwners
	keys    []affectedPrefix
}

// removePhysicalEntriesLocked tests identity under the same storage lock as
// received replacement, never against a copied page. Each callback receives
// owned native keys from RemoveFamilyMatching. Caller MUST hold peerMu and MUST
// publish keys only after releasing it. Its work/memory are bounded by this
// source owner's current native route population, with no retained history.
func (r *RIBManager) removePhysicalEntriesLocked(source ribPhysicalSource, includeFamily func(family.Family) bool, includeEntry func(*storage.RouteEntry) bool) physicalSourceRemoval {
	var result physicalSourceRemoval
	routes := r.bgpPeers[source.owner]
	if routes == nil {
		return result
	}
	for _, fam := range routes.Families() {
		if !includeFamily(fam) {
			continue
		}
		result.count += routes.RemoveFamilyMatching(fam, func(entry storage.RouteEntry) bool {
			if !source.matchesReceived(&entry) {
				return false
			}
			return includeEntry(&entry)
		}, func(raw []byte, message uint64, addPath bool) {
			result.keys = append(result.keys, affectedPrefix{fam: fam, nlriBytes: raw, addPath: addPath})
			result.removed.add(fam, raw, addPath, message)
		})
	}
	return result
}

// removePhysicalSource closes only original input and joins its selected
// output before its lease release. Every caller MUST propagate its refusal.
// RFC 4724 Section 4.2: "If the session does not get re-established within the
// "Restart Time" that the peer advertised previously, the Receiving Speaker MUST
// delete all the stale routes from the peer that it is retaining."
func (r *RIBManager) removePhysicalSource(source ribPhysicalSource, selected *family.Family, includeEntry func(*storage.RouteEntry) bool, timer *peerGRState, complete bool) (int, error) {
	if timer != nil {
		r.peerMu.RLock()
		current := r.grState[source.owner] == timer
		r.peerMu.RUnlock()
		if !current {
			return 0, nil
		}
	}
	receipts := r.sourceLeases.receiptsForPhysical(source)
	active, err := r.selectedDispatch.sourceDestinations(receipts)
	if err != nil {
		return 0, err
	}
	release := r.sourceLeases.holdSourceOutput(receipts, active)
	defer release()
	r.peerMu.Lock()
	if timer != nil {
		if r.grState[source.owner] != timer {
			r.peerMu.Unlock()
			return 0, nil
		}
	}
	if r.stopping {
		r.peerMu.Unlock()
		return 0, sourceCommandRefused("The RIB is stopping")
	}
	known := r.physicalSourceKnownLocked(source)
	if !known {
		if len(receipts) == 0 {
			r.peerMu.Unlock()
			return 0, sourceCommandRefused("The original received connection has no current or pending lifetime")
		}
	}
	if err := r.physicalSentOutputReadyLocked(source, receipts); err != nil {
		r.peerMu.Unlock()
		return 0, err
	}
	result := r.removePhysicalEntriesLocked(source, func(fam family.Family) bool {
		if selected == nil {
			return true
		}
		return fam == *selected
	}, includeEntry)
	writes := r.withdrawPhysicalRemovedSentLocked(source, result.removed)
	state := r.grState[source.owner]
	if source.ownsState(state) {
		if complete {
			r.stopGRExpiryLocked(state)
			delete(r.grState, source.owner)
			delete(r.retainedPeers, source.owner)
		}
	}
	if routes := r.bgpPeers[source.owner]; routes != nil {
		if routes.Len() == 0 {
			routes.Release()
			delete(r.bgpPeers, source.owner)
		}
	}
	r.peerMu.Unlock()
	output := r.sourceLeases.holdPhysicalSentOutput(source, receipts, writes)
	refused := r.joinPhysicalSourceOutput(output)
	// Election/publication happens while original output holds still exist.
	r.reconcileSourceKeys(result.keys)
	receipts = append(receipts, r.sourceLeases.receiptsForPhysical(source)...)
	refused = errors.Join(refused, r.settlePurgedSource(active, receipts, release))
	if refused != nil {
		return result.count, opererr.Wrap(refused, "Original received cleanup has unfinished output", "The routes were removed but their captured output or source lease has not joined", "ze request reload")
	}
	return result.count, nil
}

// withdrawPhysicalRemovedSentLocked retains the original destination receipt
// and additionally requires original received physical provenance. No new sent
// inventory is added. Caller MUST hold peerMu and later dispatch unlocked.
func (r *RIBManager) withdrawPhysicalRemovedSentLocked(source ribPhysicalSource, removed removedReceivedOwners) []sentLifecycleWrite {
	r.fenceSentLifecycleLocked(source.owner)
	var writes []sentLifecycleWrite
	for destination, destinations := range r.ribOut {
		for fam, owners := range removed {
			routes := destinations[fam]
			for key, entry := range routes {
				if !source.matchesSent(&entry) {
					continue
				}
				original, known := sentReceivedOwner(fam, key, &entry)
				if !known {
					continue
				}
				if _, gone := owners[original]; !gone {
					continue
				}
				writes = append(writes, r.sentRemoval(destination, fam, key, &entry))
				delete(routes, key)
				entry.release()
			}
			if len(routes) == 0 {
				delete(destinations, fam)
			}
		}
		if len(destinations) == 0 {
			delete(r.ribOut, destination)
		}
	}
	return writes
}

// expirePhysicalSource is the existing safety timer's original source branch.
// It MUST use the captured state's scope, not current peer metadata. The caller
// MUST pass that same state for the pointer fence against a later GR cycle.
func (r *RIBManager) expirePhysicalSource(state *peerGRState) {
	_, err := r.removePhysicalSource(*state.source, nil, func(entry *storage.RouteEntry) bool {
		return entry.StaleLevel > storage.StaleLevelFresh
	}, state, true)
	if err != nil {
		logger().Warn("original physical GR safety expiry was not completed", "error", err,
			"code", diagnostic.CodeDoctorRecentEvents, "fix", "ze request reload")
	}
}

// receiptsForPhysical snapshots current/pending rights for one exact original
// connection. No peer, election or storage lock may be held across this method.
func (s *ribSourceLeases) receiptsForPhysical(source ribPhysicalSource) []uint64 {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var receipts []uint64
	for receipt, lease := range s.leases {
		if lease.peerOwner != source.owner {
			continue
		}
		if lease.connectionID != source.connection {
			continue
		}
		receipts = append(receipts, receipt)
	}
	return receipts
}

// retainPhysical performs the same actual SDK retention operation as an
// explicit peer-wide operator command, with only the original connection's
// current leases. It creates no authority when that connection owns no lease.
func (s *ribSourceLeases) retainPhysical(ctx context.Context, source ribPhysicalSource) error {
	if s == nil {
		return nil
	}
	return s.retainReceipts(ctx, netip.Addr{}, source.owner, s.receiptsForPhysical(source))
}
