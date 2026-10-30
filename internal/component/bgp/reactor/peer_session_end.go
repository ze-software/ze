// Design: docs/architecture/behavior/fsm.md -- Reactor Notification: the leave arm and the down
// Related: peer_run.go -- runOnce registers the down and the leave arm
// Related: receive_publication.go -- the failed fact the delivery worker sets
// Related: peer_bfd.go -- the BFD release the leave arm cannot wait on
package reactor

import (
	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// sessionLeave is what the FSM leave-Established arm hands Peer.runOnce's
// down: what the close is derived from (the event, whether a NOTIFICATION was
// exchanged, the recorded close reason, the forward generation), the history
// row it owes, and the BFD release whose wait it cannot run. The arm writes
// the fields and then closes signal; the down reads them only after <-signal,
// so the close is the happens-before edge and no lock is needed. The down
// derives the close itself (sessionLeave.close), because one of its inputs,
// the failed receive publication, is final only once the delivery worker
// exited. One record per runOnce: a session's FSM leaves Established at most
// once.
type sessionLeave struct {
	signal       chan struct{}
	event        fsm.Event
	notified     bool
	notification plugin.CloseNotification
	cause        error
	generation   uint64
	transition   FSMTransition
	bfd          bfdRelease
}

// close derives the session's close from what the leave arm recorded and from
// whether the session's receive publication failed. The caller MUST read
// publicationFailed after the delivery worker exited, so a failure the worker
// meets after the leave arm ran still names the close.
func (leave *sessionLeave) close(publicationFailed bool) sessionClose {
	closed := sessionCloseFor(leave.notified, publicationFailed, leave.event, leave.cause)
	closed.generation = leave.generation
	closed.notification = leave.notification
	return closed
}

// sessionEndHooks are test seams on the session end. Each nil hook is skipped.
// Production never sets Peer.sessionEndHooks.
type sessionEndHooks struct {
	// beforeNegotiated runs after publication admission, before the phase mark.
	// It holds no locks and may pause a BFD-driven publication.
	beforeNegotiated func()
	// leaveArm runs first in the leave-Established arm, on whichever goroutine
	// drains the FSM's pending queue.
	leaveArm func()
	// beforeLeaveFire runs in the down between reading Established and firing
	// the leave event.
	beforeLeaveFire func()
	// drained runs in the down once the delivery worker has exited.
	drained func()
	// beforeHandOff runs on the read goroutine for each received UPDATE, in
	// Reactor.notifyMessageReceiver after the route-server fast path returned
	// and before the UPDATE is handed to the delivery worker. No lock is held,
	// so a test can panic there the way a defect on the read path would.
	beforeHandOff func()
	// deliveryReceiver replaces the receiver the delivery worker read, so a
	// test can hand it nil after the read goroutine enqueued an UPDATE, which
	// the read goroutine's own nil check otherwise makes unreachable.
	deliveryReceiver func(MessageReceiver) MessageReceiver
}

func (h *sessionEndHooks) receiverFor(receiver MessageReceiver) MessageReceiver {
	if h != nil && h.deliveryReceiver != nil {
		return h.deliveryReceiver(receiver)
	}
	return receiver
}

func (h *sessionEndHooks) runLeaveArm() {
	if h != nil && h.leaveArm != nil {
		h.leaveArm()
	}
}

func (h *sessionEndHooks) runBeforeLeaveFire() {
	if h != nil && h.beforeLeaveFire != nil {
		h.beforeLeaveFire()
	}
}

func (h *sessionEndHooks) runBeforeHandOff() {
	if h != nil && h.beforeHandOff != nil {
		h.beforeHandOff()
	}
}

func (h *sessionEndHooks) runDrained() {
	if h != nil && h.drained != nil {
		h.drained()
	}
}

// leaveSession is the FSM leave-Established arm. It runs on whichever
// goroutine drains the FSM's pending queue: the read goroutine, a teardown
// caller, the hold timer, the BFD subscriber or a forward-pool worker. So it
// waits on nothing those goroutines own. It marks the session closing, does
// the reactor bookkeeping, clears the encoding contexts and leaves
// PeerStateEstablished, then closes the leave signal as its last statement.
// The down, the "session closed" log line and the history row are
// sessionDown's, once the buffered UPDATEs were published.
func (p *Peer) leaveSession(session *Session, leave *sessionLeave, from, to fsm.State, event fsm.Event) {
	p.sessionEndHooks.Load().runLeaveArm()

	// Release any BFD session opened on Established, without waiting for its
	// subscriber goroutine: that goroutine may be the one running this arm
	// (a BFD Down drains the leave on it), and waiting here on its own exit
	// would never return. sessionDown waits instead.
	//
	// A strict peer keeps its session:
	// draft-ietf-idr-bgp-bfd-strict-mode Section 7 says
	// implementations "SHOULD NOT immediately destroy BFD
	// sessions when associated BGP connections transition
	// to Idle", and a peer that rebuilt the BFD handshake
	// on every failed attempt would never converge. Its
	// release is Peer.cleanup.
	if !p.bfdStrict() {
		leave.bfd = p.releaseBFDClient()
	}

	// Drop this peer out of the startup convergence hold. Without it
	// the hold's release condition would read "has been Established",
	// and a peer that came up and dropped again would keep counting
	// toward convergence while its wire is gone (update_delay.go).
	p.updateDelayPeerDown()

	// The close is derived once, in the down, from the event that made
	// this transition, from whether a NOTIFICATION crossed the wire, from
	// the close reason the session recorded, and from whether its receive
	// publication failed, and every consumer reads it: Graceful Restart
	// and BMP both decide on it (peer_close.go). The arm records the
	// inputs it owns at the transition.
	leave.event = event
	leave.notified = p.notificationExchanged.Load()
	leave.cause = closeCause(session)
	leave.generation = p.forwardGeneration.Load()
	if leave.notified {
		leave.notification = p.closingNotification()
	}

	p.mu.RLock()
	reactor := p.reactor
	p.mu.RUnlock()
	// announceClosed marks the session closing before the down reads
	// the delivery graph, so a reload racing it can place that read
	// (applyHotSwappableSettings). It runs before clearEncodingContexts,
	// which the update group index is keyed on.
	if reactor != nil {
		p.announceClosed(reactor, session)
	}

	// Clear negotiated capabilities and encoding contexts on session teardown
	p.negotiated.Store(nil)
	p.clearEncodingContexts()
	p.setState(PeerStateConnecting)

	leave.transition = FSMTransition{
		Timestamp: p.clock.Now(),
		From:      from.String(),
		To:        to.String(),
	}
	close(leave.signal)
}

// sessionDown is Peer.runOnce's down, deferred after the delivery worker
// started, so it runs on the runOnce goroutine on every exit, a read-goroutine
// panic included. In order:
//
//  1. When the FSM still reads Established, nothing else ended the session
//     (Session.Run returned on a body-read error or from its pause gate), so
//     it fires the leave itself.
//  2. It closes deliverChan and waits for the delivery worker, which
//     publishes every buffered UPDATE first.
//  3. When the FSM ever entered Established, it waits on the leave signal:
//     the leave arm may still be running on another goroutine. Then it waits
//     for the BFD subscriber the arm released, which is never this goroutine.
//  4. It derives the close, now that every failure of the session's receive
//     publication has happened (sessionLeave.close), raises session-dropped
//     when no NOTIFICATION was exchanged, tells the plugins down, logs
//     "session closed" and appends the history row.
//
// So a plugin hears every UPDATE of a session before its down, and the down
// follows PeerStateConnecting.
func (p *Peer) sessionDown(session *Session, publication *receivePublication, deliverChan chan deliveryItem, deliveryDone <-chan struct{}, leave *sessionLeave) {
	if session.fsm.State() == fsm.StateEstablished {
		p.sessionEndHooks.Load().runBeforeLeaveFire()
		// Without this event the leave arm, and so the down, would never
		// run. A local teardown that moved the FSM to Idle after the read
		// above leaves Idle in place, where this event changes nothing, so
		// exactly one leave arm runs either way.
		session.logFSMEvent(fsm.EventTCPConnectionFails)
	}

	close(deliverChan)
	<-deliveryDone
	p.deliverChan = nil
	p.sessionEndHooks.Load().runDrained()

	if !session.fsm.EnteredEstablished() {
		return
	}
	<-leave.signal
	p.awaitBFDRelease(leave.bfd)

	// The delivery worker has exited and Session.Run has returned, so no
	// fail() of this publication is still to come: a failure the worker met
	// after the leave arm ran, or after the session ended for another
	// reason, names this close too.
	closed := leave.close(publication.failed.Load())
	reason := closed.reason
	leave.transition.Reason = reason
	// Raise session-dropped on the report bus only when no NOTIFICATION
	// was exchanged during this session. If one was sent or received,
	// the operator already sees that event in `ze show errors` and a
	// duplicate session-dropped would be noise.
	if !leave.notified {
		raiseSessionDropped(p.peerAddrLabel(), reason)
	}

	p.mu.RLock()
	reactor := p.reactor
	p.mu.RUnlock()
	if reactor != nil {
		// The departed session's released receipts can never be restored.
		reactor.dropReleasedSource(p)
		p.announceDown(reactor, closed)
	}
	peerLogger().Info("session closed", "peer", p.settings.Address.String(), "reason", reason)
	p.history.append(leave.transition)
}

// beginNegotiatedPublication admits at most one publication for this session.
// The caller MUST mark negotiationDone after delivery, including panic unwinding.
// Admission adds the publisher under mu before an ending session can wait.
func (s *Session) beginNegotiatedPublication() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.negotiationEnded || s.negotiationStarted {
		return false
	}
	s.negotiationStarted = true
	s.negotiationDone.Add(1)
	return true
}

// finishNegotiatedPublication seals admission and joins an admitted publisher.
// It MUST run without peer/reactor locks: the publisher needs both to mark its
// phase before delivering. Queued FSM callbacks cannot publish after this seal.
func (s *Session) finishNegotiatedPublication() bool {
	s.mu.Lock()
	s.negotiationEnded = true
	started := s.negotiationStarted
	s.mu.Unlock()
	if !started {
		return false
	}
	s.negotiationDone.Wait()
	return true
}

// endNegotiatedSession ends only the publication made in OpenConfirm. The
// Established observers require an established session (and may remove routes),
// so only the state dispatcher is told down. runOnce calls this after delivery
// drains and before it can start the next connection.
func (p *Peer) endNegotiatedSession(session *Session) {
	if !session.finishNegotiatedPublication() {
		return
	}
	p.mu.RLock()
	reactor := p.reactor
	negotiated := p.upPhaseLocked(session) == sessionUpNegotiated
	p.mu.RUnlock()
	if !negotiated {
		return
	}
	p.markSessionUp(reactor, session, sessionUpClosing)
	if reactor.eventDispatcher == nil {
		return
	}
	closed := sessionClose{reason: rpc.ReasonTCPFailure, generation: p.forwardGeneration.Load()}
	if closeCause(session) != nil {
		closed.reason = rpc.ReasonLocalClose
	}
	if p.notificationExchanged.Load() {
		closed.reason = rpc.ReasonNotification
		closed.notification = p.closingNotification()
	}
	observer := apiStateObserver{dispatcher: reactor.eventDispatcher}
	observer.OnPeerClosed(p, closed)
}

// failReceivePublication records that the delivery worker could not publish a
// batch, and on the first failure tears the session down with a Cease carrying
// subcode: its routes would otherwise stay installed while its UPDATEs never
// reach the plugins. The caller names the subcode: Out of Resources when no
// receiver exists, Unspecific when the worker contained a panic.
// It MUST NOT wait for the delivery worker, which calls it:
// Session.teardownAutomatic sends the NOTIFICATION, closes the connection and
// signals Run, and waits on neither.
func failReceivePublication(session *Session, publication *receivePublication, subcode uint8) {
	if !publication.fail() {
		return
	}
	// RFC 4271 Section 6.7: "In the absence of any fatal errors (that are
	// indicated in this section), a BGP peer MAY choose, at any given time, to
	// close its BGP connection by sending the NOTIFICATION message with the
	// Error Code Cease."
	// RFC 4486 Section 4: "If a BGP speaker runs out of resources (e.g.,
	// memory) and decides to reset a session, then the speaker MAY send a
	// NOTIFICATION message with the Error Code Cease and the Error Subcode
	// "Out of Resources"."
	// RFC 4271 Section 4.5: "If no appropriate Error Subcode is defined, then a
	// zero (Unspecific) value is used for the Error Subcode field."
	if err := session.teardownAutomatic(subcode, ""); err != nil {
		peerLogger().Warn("receive publication failed, teardown failed",
			"peer", session.settings.Address, "error", err)
	}
}
