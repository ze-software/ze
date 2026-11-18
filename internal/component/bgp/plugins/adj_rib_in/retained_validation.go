// Design: docs/architecture/plugin/rib-storage-design.md -- GR validation lifetimes.
// Related: source_state.go -- exact original physical DOWN; rib_validation.go -- gate publication.
package adj_rib_in

import (
	"encoding/binary"
	"encoding/hex"

	"github.com/ze-software/ze/internal/component/bgp/retention"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/family"
)

// handleRetainedValidationEvent runs on the existing SDK event rail. A global
// pre-DOWN verdict records only a state lifetime this process actually owns;
// cleanup matches actual stored owner/connection facts and cannot create data.
func (r *AdjRIBInManager) handleRetainedValidationEvent(delivered string) (bool, error) {
	if envelope, matched := retention.Accepts(delivered); matched {
		return true, r.retentionVerdicts.Record(envelope)
	}
	envelope, matched := retention.AcceptsCleanup(delivered)
	if !matched {
		return false, nil
	}
	cleanup, err := retention.DecodeCleanup(envelope)
	if err != nil {
		return true, err
	}
	// Direct replay commands run beside event callbacks. Take this writer
	// before mutationMu: every already-built self-replay reaches the engine
	// before cleanup closes admission; later snapshots see the removed rows.
	r.replayGuard.Lock()
	defer r.replayGuard.Unlock()
	r.mutationMu.Lock()
	defer r.mutationMu.Unlock()
	r.mu.Lock()
	defer r.unlockValidation()
	source := receivedPeer{address: cleanup.Address(), owner: cleanup.Owner()}
	return true, r.cleanupValidationConnectionLocked(source, cleanup)
}

// retireValidationConnectionLocked is the actual received-store DOWN action.
// RFC 4724 Section 4.2: "the Receiving Speaker MUST retain the routes
// received from the peer for all the address families that were previously
// received in the Graceful Restart Capability and MUST mark them as stale
// routing information."
// Retention preserves prior validation outcomes; it never makes a route valid.
// Caller MUST hold mu and finish with unlockValidation through publication.
func (r *AdjRIBInManager) retireValidationConnectionLocked(source receivedPeer, connection uint64, keep map[family.Family]bool) {
	var removed []compactRouteKey
	if routes := r.ribIn[source]; routes != nil {
		routes.Range(func(key compactRouteKey, _ uint64, route *RawRoute) bool {
			if route.ConnectionID != connection {
				return true
			}
			if keep[key.Fam] {
				route.retainedForValidation = true
				return true
			}
			if route.retainedForValidation {
				return true
			}
			removed = append(removed, key)
			return true
		})
	}
	for _, key := range removed {
		r.removeInstalled(source, key)
	}
	for key, route := range r.pending {
		if key.Peer != source {
			continue
		}
		if route.route.ConnectionID != connection {
			continue
		}
		if keep[key.Route.Fam] {
			route.route.retainedForValidation = true
			continue
		}
		if route.route.retainedForValidation {
			continue
		}
		r.removePending(source, key.Route)
	}
	for key, decision := range r.earlyDecisions {
		if key.Peer != source {
			continue
		}
		if decision.connectionID != connection {
			continue
		}
		if !keep[key.Route.Fam] {
			delete(r.earlyDecisions, key)
		}
	}
	r.pruneValidationMetadataLocked(source)
}

// pruneValidationMetadataLocked retains only metadata referenced by actual
// current installed/pending bytes. Caller MUST hold mu through publication.
func (r *AdjRIBInManager) pruneValidationMetadataLocked(source receivedPeer) {
	for key := range r.validationPeers {
		if key.source != source {
			continue
		}
		if !r.validationPeerPresentLocked(key) {
			delete(r.validationPeers, key)
		}
	}
	if _, live := r.sourceLive[source.owner]; live {
		return
	}
	if routes := r.ribIn[source]; routes != nil {
		if routes.Len() != 0 {
			return
		}
	}
	for key := range r.pending {
		if key.Peer == source {
			return
		}
	}
	r.dropSourcePolicyLocked(source.owner)
}

// rawNoLLGR reads original authoritative bytes, not the later LLGR_STALE
// decoration. An invalid hex carrier refuses cleanup before any mutation.
func rawNoLLGR(route *RawRoute) (bool, error) {
	raw, err := hex.DecodeString(route.AttrHex)
	if err != nil {
		return false, err
	}
	iterator := attribute.NewAttrIterator(raw)
	for code, _, value, ok := iterator.Next(); ok; code, _, value, ok = iterator.Next() {
		if code != attribute.AttrCommunity {
			continue
		}
		for off := 0; off+4 <= len(value); off += 4 {
			if attribute.Community(binary.BigEndian.Uint32(value[off:])) == attribute.CommunityNoLLGR {
				return true, nil
			}
		}
	}
	return false, nil
}

// cleanupValidationConnectionLocked closes admission before the final source
// purge. It removes only actual old physical entries; successor and sibling
// values cannot be selected by address. Caller MUST hold mu and publish after.
func (r *AdjRIBInManager) cleanupValidationConnectionLocked(source receivedPeer, cleanup retention.Cleanup) error {
	selected := make(map[compactRouteKey]struct{})
	var failure error
	visit := func(key compactRouteKey, route *RawRoute) {
		if route.ConnectionID != cleanup.Connection() {
			return
		}
		if !cleanup.MatchesFamily(key.Fam) {
			return
		}
		if cleanup.NoLLGROnly() {
			present, err := rawNoLLGR(route)
			if err != nil {
				failure = err
				return
			}
			if !present {
				return
			}
		}
		selected[key] = struct{}{}
	}
	if routes := r.ribIn[source]; routes != nil {
		routes.Range(func(key compactRouteKey, _ uint64, route *RawRoute) bool { visit(key, route); return failure == nil })
	}
	for key, route := range r.pending {
		if key.Peer == source {
			visit(key.Route, route.route)
		}
	}
	if failure != nil {
		return failure
	}
	for key := range selected {
		r.removeInstalled(source, key)
		r.removePending(source, key)
	}
	if !cleanup.NoLLGROnly() {
		for key, decision := range r.earlyDecisions {
			if key.Peer != source {
				continue
			}
			if decision.connectionID != cleanup.Connection() {
				continue
			}
			if cleanup.MatchesFamily(key.Route.Fam) {
				delete(r.earlyDecisions, key)
			}
		}
	}
	r.pruneValidationMetadataLocked(source)
	return nil
}
