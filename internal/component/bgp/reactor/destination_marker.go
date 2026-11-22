// Design: docs/architecture/api/process-protocol.md -- constrained legacy replay markers.
// Related: destination_constraint.go -- exact Peer and final writer narrowing.
package reactor

import (
	"context"
	"errors"
	"slices"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/clock"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// SendBoundRefresh completes one original-session marker before returning. It
// shares the existing refresh grant and negotiated capability rules, and waits
// only for this destination's previously admitted forwards. It never queues a
// marker into a successor's initial replay. Safe for concurrent use.
func (a *reactorAPIAdapter) SendBoundRefresh(ctx context.Context, sel *selector.Selector, afi uint16, safi uint8, action rpc.SelectedRefreshAction, sender plugin.Sender) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	owner, replay := sender.Destination()
	if owner == 0 {
		return errAdjOutSession
	}
	lifetime, err := plugin.OriginProducerFromContext(ctx)
	if err != nil {
		return err
	}
	if !lifetime.Sender().SameIssuer(sender) {
		return errAdjOutProvenance
	}
	if err := lifetime.Current(); err != nil {
		return err
	}
	process, ok := lifetime.ProcessContext()
	if !ok {
		return errAdjOutProvenance
	}
	work, cancel := context.WithCancel(ctx)
	stop := context.AfterFunc(process, cancel)
	defer func() { stop(); cancel() }()
	ctx = work
	subtype := message.RouteRefreshBoRR
	switch action {
	case rpc.SelectedRefreshBegin:
	case rpc.SelectedRefreshEnd:
		subtype = message.RouteRefreshEoRR
	case rpc.SelectedRefreshUnspecified:
		return errAdjOutSession
	default:
		return errAdjOutSession // Open external action input.
	}
	a.r.mu.RLock()
	peer, session, err := a.matchCapturedDestination(sel, sender)
	a.r.mu.RUnlock()
	if err != nil {
		return err
	}
	if err := destinationCurrent(peer, session, sender); err != nil {
		return err
	}
	target := announceTarget{peer: peer, session: session, destinationOwner: owner, destinationSession: replay}
	if err := target.settlePriorDestinationWrites(ctx); err != nil {
		return err
	}
	ordinary, err := a.boundOrdinaryMarkerOwner(peer, session, lifetime)
	if err != nil {
		return err
	}
	if ordinary {
		deadline, ok := ctx.Deadline()
		if !ok {
			clk := a.r.clock
			if clk == nil {
				clk = clock.RealClock{}
			}
			deadline = clk.Now().Add(rpc.CancellableRequestTimeout)
		}
		request, err := plugin.NewSelectedRefresh(rpc.SelectRefreshInput{Destination: peer.addrString,
			PeerOwner: owner, DestinationSession: replay, Family: family.Family{AFI: family.AFI(afi), SAFI: family.SAFI(safi)}.String(),
			Action: action, DeadlineMS: deadline.UnixMilli()})
		if err != nil {
			return err
		}
		_, err = a.SelectRefresh(ctx, request, lifetime)
		return err
	}
	a.r.mu.RLock()
	_, refused := filterPermittedPeers([]*Peer{peer}, refreshOrigin(sender))
	a.r.mu.RUnlock()
	if refused != 0 {
		return sendNotPermitted("original destination", refreshOrigin(sender))
	}
	negotiated := session.Negotiated()
	if negotiated == nil {
		return errAdjOutSession
	}
	fam := family.Family{AFI: family.AFI(afi), SAFI: family.SAFI(safi)}
	if !slices.Contains(negotiated.Families(), fam) {
		return ErrFamilyNotNegotiated
	}
	if !negotiated.EnhancedRouteRefresh {
		return nil
	}
	marker := &message.RouteRefresh{AFI: message.AFI(afi), SAFI: message.SAFI(safi), Subtype: subtype}
	if err := session.sendBoundMarker(ctx, marker, replay, lifetime, peer); err != nil {
		return err
	}
	peer.incrRefreshSent()
	return nil
}

// sendBoundMarker MUST keep the original token through writeMu acquisition.
// It consumes only a locally built ROUTE-REFRESH packet and returns actual
// socket/flush completion, so cancellation never acknowledges queued work.
func (s *Session) sendBoundMarker(ctx context.Context, marker *message.RouteRefresh, replay uint64, owner plugin.SelectedOwner, peer *Peer) error {
	if err := s.selectedMu.LockContext(ctx); err != nil {
		return err
	}
	defer s.selectedMu.Unlock()
	if err := s.acquireWriteContext(ctx, s.controlWriteDeadline()); err != nil {
		return err
	}
	defer s.releaseWrites()
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := s.checkDestinationMarker(replay, peer); err != nil {
		return err
	}
	if err := owner.Current(); err != nil {
		return err
	}
	fam := family.Family{AFI: marker.AFI, SAFI: marker.SAFI}
	if err := s.selectedRefreshOwner(fam, owner); err != nil {
		return err
	}
	// RFC 7313 Section 4. The actual writer, not a configured GR flag or
	// staged UPDATE, records the EOR accepted by this original connection.
	if marker.Subtype == message.RouteRefreshBoRR {
		if s.localGR {
			if s.wireWriter == nil {
				return errors.New("bound refresh has no completed EOR record; finish this session's initial routing update before refresh")
			}
			if !s.wireWriter.sentEOR[fam] {
				return errors.New("bound refresh EOR has not reached this session's wire; finish this family's initial routing update before refresh")
			}
		}
	}
	conn := s.conn
	if conn == nil {
		return ErrNotConnected
	}
	cleanup, err := s.requestWriteDeadline(ctx, conn)
	if err != nil {
		return err
	}
	defer cleanup()
	if err := s.writeMessageHeld(marker); err != nil {
		if canceled := ctx.Err(); canceled != nil {
			dropConn(conn)
			return canceled
		}
		return err
	}
	action := rpc.SelectedRefreshBegin
	if marker.Subtype == message.RouteRefreshEoRR {
		action = rpc.SelectedRefreshEnd
	}
	s.recordSelectedRefresh(conn, fam, action, owner)
	return nil
}

// checkDestinationMarker runs only with writeMu held. The immutable captured
// Session and its token jointly prevent old markers reaching a successor.
func (s *Session) checkDestinationMarker(replay uint64, peer *Peer) error {
	if peer.establishedSession.Load() != s {
		return errAdjOutSession
	}
	if replay == 0 {
		return errAdjOutSession
	}
	if s.tearingDown.Load() {
		return errAdjOutSession
	}
	if s.initialReplay.Load() != replay {
		return errAdjOutSession
	}
	return nil
}

// boundOrdinaryMarkerOwner answers only from this original destination's
// published ordinary owner and the actual current Process incarnation. Failure
// after identifying that owner MUST refuse, never fall through to a weaker rail.
func (a *reactorAPIAdapter) boundOrdinaryMarkerOwner(peer *Peer, session *Session, owner plugin.SelectedOwner) (bool, error) {
	snapshot := peer.selectedOwnerSnapshot()
	if snapshot == nil {
		return false, nil
	}
	if snapshot.err != nil {
		return false, snapshot.err
	}
	if snapshot.peer != peer {
		return false, errAdjOutSession
	}
	if snapshot.session != session {
		return false, errAdjOutSession
	}
	if !snapshot.owner.IsSet() {
		return false, nil
	}
	if snapshot.owner.Explicit() {
		return false, nil
	}
	name, named := owner.Sender().Process()
	if !named {
		return false, errAdjOutProvenance
	}
	if snapshot.owner.Name() != name {
		return false, nil
	}
	if a.r.api == nil {
		return false, errAdjOutSession
	}
	manager := a.r.api.ProcessManager()
	if manager == nil {
		return false, errAdjOutSession
	}
	current := manager.GetProcess(name)
	if current == nil {
		return false, errAdjOutSession
	}
	if !current.Running() {
		return false, errAdjOutSession
	}
	if current.Stage() < plugin.StageRunning {
		return false, errAdjOutSession
	}
	captured, err := current.SelectedOwner()
	if err != nil {
		return false, err
	}
	if !captured.SameIncarnation(owner) {
		return false, errAdjOutProvenance
	}
	if peer.selectedOwnerSnapshot() != snapshot {
		return false, errAdjOutSession
	}
	if peer.currentSession() != session {
		return false, errAdjOutSession
	}
	return true, nil
}
