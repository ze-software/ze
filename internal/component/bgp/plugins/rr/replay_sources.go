// Design: docs/architecture/plugin/rib-storage-design.md -- original source catch-up before replay.
// Related: reflect.go -- native and text ingress publish applied UPDATE IDs.
package rr

import (
	"context"
	"fmt"

	"github.com/ze-software/ze/internal/core/bgp/ribevents"
)

// captureReplaySources runs on the ordered state callback before replay I/O.
// Its bounds are the physical sources RR can reflect to the new destination.
// Holding withdrawalMu before mu is the order used by stored admission.
func (rr *routeReflector) captureReplaySources(target peerTarget) {
	rr.withdrawalMu.Lock()
	defer rr.withdrawalMu.Unlock()
	rr.mu.Lock()
	defer rr.mu.Unlock()
	destination := rr.peers[target.identity]
	if destination == nil {
		return
	}
	for source, state := range rr.peers {
		if source.owner == target.identity.owner {
			continue
		}
		if !state.Up {
			continue
		}
		if state.processedMsgID == 0 {
			continue
		}
		if state.Internal && !state.RRClient && !destination.RRClient {
			continue
		}
		destination.replaySources = append(destination.replaySources, ribevents.ReplaySource{
			Address: source.address, PeerOwner: source.owner, ConnectionID: source.connection, MessageID: state.processedMsgID,
		})
	}
}

// waitReplaySources runs before the first output. A retry can rerun this
// projection without repeating any partially completed write from this wait.
// A source that ended no longer owes live progress. Its historical stored
// paths still require ordinary exact retained membership to be replayed.
func (rr *routeReflector) waitReplaySources(ctx context.Context, target peerTarget, sources []ribevents.ReplaySource) error {
	var pending []ribevents.ReplaySource
	for len(sources) != 0 {
		if !rr.targetCurrent(target) {
			return nil
		}
		pending = pending[:0]
		rr.mu.RLock()
		for _, source := range sources {
			state := rr.peers[peerIdentity{owner: source.PeerOwner, connection: source.ConnectionID, address: source.Address}]
			if state != nil && state.Up {
				pending = append(pending, source)
			}
		}
		rr.mu.RUnlock()
		covered, err := ribevents.ReadReplaySourceProgress(ctx, rr.dispatchCommand, target.identity.address.String(), pending)
		if err != nil {
			return fmt.Errorf("bgp-rr replay source progress: %w", err)
		}
		if covered {
			return nil
		}
		if err := waitStoredEligibility(ctx); err != nil {
			return fmt.Errorf("bgp-rr replay source progress: %w", err)
		}
	}
	return nil
}
