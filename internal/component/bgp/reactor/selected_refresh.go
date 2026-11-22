// Design: docs/architecture/update-building.md -- exact selected refresh markers.
// Related: reactor_api_selected.go -- selecting process and destination authority.
// RFC 7313 Section 4 -- see rfc/short/rfc7313.md.
package reactor

import (
	"context"
	"errors"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// SelectRefresh admits one marker on the same exact selected stream as routes.
// The selecting worker MUST await Begin, all replay receipts, then End. This
// operation MUST NOT itself enumerate candidates or complete queued replay work.
func (a *reactorAPIAdapter) SelectRefresh(ctx context.Context, request *plugin.SelectedRefresh, owner plugin.SelectedOwner) (*rpc.SelectRefreshOutput, error) {
	if request == nil {
		return nil, errors.New("select-refresh validated request is missing; construct the declared request")
	}
	input := request.Input()
	ctx, cancel := rpc.SelectRefreshCallContext(ctx, &input)
	defer cancel()
	if err := validateSelectedOwner(owner); err != nil {
		return nil, err
	}
	lifetime, bound := owner.ProcessContext()
	if !bound {
		return nil, errors.New("select-refresh selecting connection lifetime is missing; reconnect the selecting process")
	}
	work, stop := context.WithCancel(ctx)
	release := context.AfterFunc(lifetime, stop)
	defer func() { release(); stop() }()
	peer, session, err := a.selectedDestination(request.Destination(), input.DestinationSession, owner)
	if err != nil {
		return nil, err
	}
	if peer.sourceOwner != input.PeerOwner {
		return nil, errors.New("select-refresh peer owner changed; reconcile from the captured peer's current UP")
	}
	if err := session.selectedMu.LockContext(work); err != nil {
		return nil, err
	}
	defer session.selectedMu.Unlock()
	snapshot, _, err := selectedExportKind(peer, session, owner)
	if err != nil {
		return nil, err
	}
	negotiated := peer.negotiated.Load()
	if negotiated == nil {
		return nil, errAdjOutSession
	}
	if !negotiated.Has(request.Family()) {
		return nil, ErrFamilyNotNegotiated
	}
	if err := session.acquireWriteContext(work, session.controlWriteDeadline()); err != nil {
		return nil, err
	}
	defer session.releaseWrites()
	if peer.selectedOwnerSnapshot() != snapshot {
		return nil, errAdjOutSession
	}
	if snapshot.session != session {
		return nil, errAdjOutSession
	}
	if session.initialReplay.Load() != input.DestinationSession {
		return nil, errAdjOutSession
	}
	if peer.State() != PeerStateEstablished {
		return nil, errAdjOutSession
	}
	if session.fsm.State() != fsm.StateEstablished {
		return nil, ErrInvalidState
	}
	if err := owner.Current(); err != nil {
		return nil, err
	}
	if err := work.Err(); err != nil {
		return nil, err
	}
	output := &rpc.SelectRefreshOutput{Destination: input.Destination, PeerOwner: input.PeerOwner,
		DestinationSession: input.DestinationSession, Family: input.Family, Action: input.Action,
		Outcome: rpc.SelectedRefreshNotNegotiated}
	if !negotiated.EnhancedRouteRefresh {
		return output, nil
	}
	if err := session.selectedRefreshOwner(request.Family(), owner); err != nil {
		return nil, err
	}
	// RFC 7313 Section 4: "For a BGP speaker that supports the BGP Graceful Restart, it MUST NOT send a BoRR for an <AFI, SAFI> to a neighbor before it sends the EoR for the <AFI, SAFI> to the neighbor."
	if input.Action == rpc.SelectedRefreshBegin {
		if err := session.checkBoRRAllowedHeld(request.Family()); err != nil {
			return nil, err
		}
	}
	conn := session.conn
	if conn == nil {
		return nil, ErrNotConnected
	}
	cleanup, err := session.requestWriteDeadline(work, conn)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	subtype := message.RouteRefreshBoRR
	if input.Action == rpc.SelectedRefreshEnd {
		subtype = message.RouteRefreshEoRR
	}
	marker := &message.RouteRefresh{AFI: request.Family().AFI,
		SAFI: request.Family().SAFI, Subtype: subtype}
	// RFC 7313 Section 4: "Before the speaker starts a route refresh that is either initiated locally, or in response to a \"normal route refresh request\" from the peer, the speaker MUST send a BoRR message."
	// RFC 7313 Section 4: "After the speaker completes the re-advertisement of the entire Adj-RIB-Out to the peer, it MUST send an EoRR message."
	if err := session.writeMessageHeld(marker); err != nil {
		if canceled := work.Err(); canceled != nil {
			dropConn(conn)
			return nil, canceled
		}
		return nil, err
	}
	peer.incrRefreshSent()
	session.recordSelectedRefresh(conn, request.Family(), input.Action, owner)
	output.Outcome = rpc.SelectedRefreshWritten
	return output, nil
}
