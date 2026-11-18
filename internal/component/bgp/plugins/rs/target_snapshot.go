// Design: docs/architecture/api/process-protocol.md -- captured role output destinations.
// Related: server_forward.go -- one cached fanout and acknowledgement.
package rs

import (
	"context"
	"errors"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

type replayTargetKey struct{}

// targetOf is called only under mu on an established original actor. A missing
// output receipt refuses automatic output rather than selecting by address.
func targetOf(key workerKey, peer *PeerState) (rpc.ForwardTarget, error) {
	if key.owner == 0 {
		return rpc.ForwardTarget{}, errors.New("route-server destination has no original owner")
	}
	if peer.DestinationSession == 0 {
		return rpc.ForwardTarget{}, errors.New("route-server destination UP carried no output receipt")
	}
	return rpc.ForwardTarget{Destination: key.address.String(), DestinationOwner: key.owner, DestinationSession: peer.DestinationSession}, nil
}

func (rs *routeServer) relayToTarget(ctx context.Context, target rpc.ForwardTarget, routes []rpc.StoredRoute) (result error) {
	if target.DestinationOwner == 0 {
		return errors.New("route-server relay has no captured destination owner")
	}
	if target.DestinationSession == 0 {
		return errors.New("route-server relay has no captured destination session")
	}
	reservations, prepareErr := rs.prepareStoredSources(ctx, routes)
	if prepareErr != nil {
		return prepareErr
	}
	reservationsHeld := true
	defer func() {
		if reservationsHeld {
			result = errors.Join(result, rs.finishStoredReservations(ctx, reservations))
		}
	}()
	var filtered []rpc.StoredRoute
	var err error
	for {
		checked, checkErr := rs.checkColdRoutes(ctx, routes)
		if checkErr != nil {
			return checkErr
		}
		rs.withdrawalMu.Lock()
		filtered, err = rs.admitStoredRoutesLocked(ctx, routes, checked)
		if errors.Is(err, errColdRevisionChanged) {
			rs.withdrawalMu.Unlock()
			continue
		}
		break
	}
	if err != nil {
		rs.withdrawalMu.Unlock()
		return err
	}
	if len(filtered) == 0 {
		rs.withdrawalMu.Unlock()
		return nil
	}
	routes = filtered
	done := make(chan struct{})
	work, admitted := rs.outputs.begin(ctx, done)
	if !admitted {
		rs.withdrawalMu.Unlock()
		return errors.New("route-server output admission is stopped")
	}
	rs.outputMu.Lock()
	if rs.outputTargets == nil {
		rs.outputTargets = make(map[chan struct{}][]rpc.ForwardTarget)
	}
	rs.outputTargets[done] = []rpc.ForwardTarget{target}
	rs.outputMu.Unlock()
	rs.withdrawalMu.Unlock()
	defer func() {
		result = errors.Join(result, rs.finishStoredReservations(ctx, reservations))
		reservationsHeld = false
		rs.rememberRelayResult(target, routes, result)
		rs.outputMu.Lock()
		delete(rs.outputTargets, done)
		rs.outputMu.Unlock()
		rs.outputs.finish(done)
	}()
	ctx = work
	if rs.relayStoredRouteHook != nil {
		return rs.relayStoredRouteHook(target.Destination, routes)
	}
	return rs.plugin.RelayStoredRoute(ctx, rpc.RelayStoredRouteInput{Destination: target.Destination, DestinationOwner: target.DestinationOwner, DestinationSession: target.DestinationSession, Routes: routes})
}
