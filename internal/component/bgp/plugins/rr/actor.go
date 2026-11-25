// Design: docs/architecture/plugin/rib-storage-design.md -- original reflector sessions.
// Related: rr.go -- ordered state delivery; source_cleanup.go -- finite applied join.
package rr

import (
	"context"
	"errors"
	"net/netip"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// peerIdentity is one original physical session. A parsed address alone cannot
// construct it. It is comparable and has no mutable current-session lookup.
type peerIdentity struct {
	owner, connection uint64
	address           netip.Addr
}

func newPeerIdentity(address string, owner, connection uint64) (peerIdentity, error) {
	if owner == 0 {
		return peerIdentity{}, errors.New("bgp-rr: received event has no original peer owner")
	}
	if connection == 0 {
		return peerIdentity{}, errors.New("bgp-rr: received event has no original physical connection")
	}
	parsed, err := netip.ParseAddr(address)
	if err != nil {
		return peerIdentity{}, err
	}
	return peerIdentity{owner: owner, connection: connection, address: parsed}, nil
}

// peerTarget is captured under mu. The writer MUST receive these same values,
// even if DOWN/reconnect occurs after the local admission check.
type peerTarget struct {
	identity            peerIdentity
	session, generation uint64
}

func (rr *routeReflector) sourcePolicyActor(owner, generation uint64) (peerIdentity, bool) {
	if owner == 0 {
		return peerIdentity{}, false
	}
	if generation == 0 {
		return peerIdentity{}, false
	}
	rr.mu.RLock()
	defer rr.mu.RUnlock()
	for identity, state := range rr.peers {
		if identity.owner != owner {
			continue
		}
		if state.PublishedGeneration != generation {
			continue
		}
		if state.Up {
			return identity, true
		}
	}
	return peerIdentity{}, false
}

func (rr *routeReflector) targets(source peerIdentity, fam family.Family) []peerTarget {
	targets, _ := rr.targetsWithHold(source, fam)
	return targets
}

func (rr *routeReflector) targetsWithHold(source peerIdentity, fam family.Family) ([]peerTarget, bool) {
	rr.mu.RLock()
	defer rr.mu.RUnlock()
	src := rr.peers[source]
	targets := make([]peerTarget, 0, len(rr.peers))
	anyHeld := false
	for identity, state := range rr.peers {
		if identity.owner == source.owner {
			continue
		}
		if !state.Up {
			continue
		}
		if !state.Internal {
			continue
		}
		// RFC 4456 Section 6: a route from an internal non-client is reflected
		// to clients only, never to another internal non-client.
		if src != nil && src.Internal && !src.RRClient && !state.RRClient {
			continue
		}
		if state.DestinationSession == 0 {
			continue
		}
		if state.UpdateSendDenied {
			continue
		}
		if fam.AFI != 0 {
			if !state.Families[fam] {
				continue
			}
		}
		if state.awaitingSessionReady {
			anyHeld = true
		}
		targets = append(targets, peerTarget{identity: identity, session: state.DestinationSession, generation: state.ReplayGen})
	}
	return targets, anyHeld
}

func (rr *routeReflector) targetCurrent(target peerTarget) bool {
	rr.mu.RLock()
	defer rr.mu.RUnlock()
	state := rr.peers[target.identity]
	if state == nil {
		return false
	}
	if !state.Up {
		return false
	}
	if !state.Internal {
		return false
	}
	if state.DestinationSession != target.session {
		return false
	}
	if state.UpdateSendDenied {
		return false
	}
	return state.ReplayGen == target.generation
}

// acquireSourceLease runs only on ordered live input, before its physical DOWN.
// Ordered received input and current live page adoption use this method.
// It MUST NOT acquire a new lease after this original actor is DOWN.
func (rr *routeReflector) acquireSourceLease(ctx context.Context, source peerIdentity, receipt uint64) error {
	if receipt == 0 {
		return errors.New("bgp-rr: received route has no original source receipt")
	}
	rr.forwards.enter()
	defer rr.forwards.leave()
	rr.mu.RLock()
	state := rr.peers[source]
	rr.mu.RUnlock()
	if state == nil {
		return errors.New("bgp-rr: source has no original UP actor")
	}
	state.leaseCall.Lock()
	defer state.leaseCall.Unlock()
	rr.mu.RLock()
	current := rr.peers[source] == state
	live, leased, same := state.Up, state.Leased, state.SourceReceipt == receipt
	rr.mu.RUnlock()
	if !current {
		return errors.New("bgp-rr: source actor was replaced")
	}
	if !live {
		return nil
	}
	if leased {
		if !same {
			return errors.New("bgp-rr: source receipt changed on its physical connection")
		}
		return nil
	}
	rr.mu.Lock()
	state.SourceReceipt = receipt
	rr.mu.Unlock()
	if _, err := rr.plugin.SourceLease(ctx, rpc.SourceLeaseInput{Action: rpc.SourceLeaseAcquire, PeerOwner: source.owner, SourceReceipt: receipt}); err != nil {
		rr.reportUpdateFailure(source.address.String(), "source lease acquire", err)
		return err
	}
	// DOWN never waits for this I/O. The acquired original lease remains owed
	// even if DOWN sealed admission while its acquire answer was in flight.
	rr.mu.Lock()
	state.SourceReceipt = receipt
	state.Leased = true
	rr.mu.Unlock()
	return nil
}

func (rr *routeReflector) relayToPeer(ctx context.Context, target peerTarget, _ string, routes []rpc.StoredRoute) error {
	if !rr.targetCurrent(target) {
		return nil
	}
	rr.mu.RLock()
	targetState := rr.peers[target.identity]
	targetClient := targetState != nil && targetState.RRClient
	rr.mu.RUnlock()
	admitted := make([]rpc.StoredRoute, 0, len(routes))
	defer func() {
		for range admitted {
			rr.forwards.leave()
		}
	}()
	for _, route := range routes {
		if !targetClient {
			if srcID, idErr := newPeerIdentity(route.SourcePeer, route.SourceOwner, route.SourceConnectionID); idErr == nil {
				rr.mu.RLock()
				srcState := rr.peers[srcID]
				blocked := srcState != nil && srcState.Internal && !srcState.RRClient
				rr.mu.RUnlock()
				if blocked {
					continue
				}
			}
		}
		accepted, err := rr.admitStoredRoute(ctx, route)
		if err != nil {
			return &storedAdmissionError{cause: err}
		}
		if accepted {
			admitted = append(admitted, route)
		}
	}
	if len(admitted) == 0 {
		return nil
	}
	if !rr.targetCurrent(target) {
		return nil
	}
	err := rr.plugin.RelayStoredRoute(ctx, rpc.RelayStoredRouteInput{Destination: target.identity.address.String(), Routes: admitted,
		DestinationOwner: target.identity.owner, DestinationSession: target.session})
	for _, route := range admitted {
		if err != nil {
			source, identityErr := newPeerIdentity(route.SourcePeer, route.SourceOwner, route.SourceConnectionID)
			if identityErr != nil {
				rr.noteWorkFailure("stored relay original identity", identityErr)
				continue
			}
			rr.noteForwardFailure("stored relay", err, source)
		}
	}
	return err
}
