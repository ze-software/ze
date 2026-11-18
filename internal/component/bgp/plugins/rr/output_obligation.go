// Design: docs/architecture/plugin/rib-storage-design.md -- original output obligations.
// Related: source_cleanup.go -- exact lease Release joins source-owned writers.
package rr

import (
	"context"
	"errors"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func (rr *routeReflector) noteSourceOutputFailure(source peerIdentity, work string, err error) {
	if err == nil {
		return
	}
	rr.mu.Lock()
	defer rr.mu.Unlock()
	state := rr.peers[source]
	if state == nil {
		return
	}
	if state.sourceOutputDebt == nil {
		state.sourceOutputDebt = errors.New("bgp-rr: original source " + source.address.String() + " " + work + " remains owed: " + err.Error())
	}
}

func (rr *routeReflector) noteTargetOutputFailure(target peerTarget, work string, err error) {
	if err == nil {
		return
	}
	rr.mu.Lock()
	defer rr.mu.Unlock()
	state := rr.peers[target.identity]
	if state == nil {
		return
	}
	if state.targetOutputDebt == nil {
		state.targetOutputDebt = errors.New("bgp-rr: original destination " + target.identity.address.String() + " " + work + " remains owed: " + err.Error())
	}
}

// reconcileEndedOutput may retire only a non-retained, ended original source.
// The local forwards gate is joined by its caller. Ended physical delivery
// refuses a delayed unadmitted beginLive; Release invalidates this process's
// exact receipt lease and waits all enrolled carriers. A live source's Release,
// a generic cancel ACK, or another receipt's success proves none of this.
func (rr *routeReflector) reconcileEndedOutput(ctx context.Context) error {
	rr.mu.RLock()
	pending := make([]peerIdentity, 0)
	for source, state := range rr.peers {
		if state.Up {
			continue
		}
		if !state.DownSeen {
			continue
		}
		if state.sourceOutputDebt == nil {
			continue
		}
		pending = append(pending, source)
	}
	rr.mu.RUnlock()
	for _, source := range pending {
		rr.mu.RLock()
		state := rr.peers[source]
		rr.mu.RUnlock()
		if state == nil {
			continue
		}
		state.leaseCall.Lock()
		rr.withdrawalMu.Lock()
		retained := false
		for key, info := range rr.withdrawals[source.address.String()] {
			if key.owner != source.owner {
				continue
			}
			if key.connection != source.connection {
				continue
			}
			if !info.Closing {
				retained = true
				break
			}
		}
		rr.withdrawalMu.Unlock()
		if retained {
			state.leaseCall.Unlock()
			continue
		}
		rr.mu.RLock()
		receipt := state.SourceReceipt
		live := state.Up
		rr.mu.RUnlock()
		if live {
			state.leaseCall.Unlock()
			continue
		}
		if receipt == 0 {
			state.leaseCall.Unlock()
			return errors.New("bgp-rr: ended original output has no source receipt to settle")
		}
		_, err := rr.plugin.SourceLease(ctx, rpc.SourceLeaseInput{Action: rpc.SourceLeaseRelease, PeerOwner: source.owner, SourceReceipt: receipt})
		if err == nil {
			rr.mu.Lock()
			state.sourceOutputDebt = nil
			state.Leased = false
			rr.mu.Unlock()
		}
		state.leaseCall.Unlock()
		if err != nil {
			return err
		}
	}
	return nil
}

// reconcileOriginalDepartures reissues only work never dispatched because a
// previous original source writer was unknown. Its original destinations stay
// captured. An attempted recovery failure is not classified as a safe retry.
func (rr *routeReflector) reconcileOriginalDepartures(ctx context.Context) error {
	rr.mu.RLock()
	pending := make(map[peerIdentity]*peerState)
	for source, state := range rr.peers {
		if state.departureOwed != nil {
			pending[source] = state
		}
	}
	rr.mu.RUnlock()
	for source, state := range pending {
		state.departureCall.Lock()
		rr.mu.RLock()
		work := state.departureOwed
		rr.mu.RUnlock()
		if work == nil {
			state.departureCall.Unlock()
			continue
		}
		if err := rr.unsettledWork(); err != nil {
			state.departureCall.Unlock()
			return err
		}
		if err := rr.retainSource(ctx, source); err != nil {
			state.departureCall.Unlock()
			return err
		}
		err := rr.sendPeerDown(ctx, source, *work)
		if err != nil {
			state.departureCall.Unlock()
			return err
		}
		rr.mu.Lock()
		state.departureOwed = nil
		rr.mu.Unlock()
		state.departureCall.Unlock()
	}
	return nil
}
