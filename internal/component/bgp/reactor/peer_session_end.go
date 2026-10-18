// Design: docs/architecture/behavior/fsm.md -- Reactor Notification: the leave arm and the down
// Related: peer_run.go -- runOnce registers the down and the leave arm
// Related: receive_publication.go -- the failed fact the delivery worker sets
// Related: peer_bfd.go -- the BFD release the leave arm cannot wait on
package reactor

import (
	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
)

// sessionLeave is what the FSM leave-Established arm hands Peer.runOnce's
// down: the close it derived, whether a NOTIFICATION was exchanged, the
// history row it owes, and the BFD release whose wait it cannot run. The arm
// writes the fields and then closes signal; the down reads them only after
// <-signal, so the close is the happens-before edge and no lock is needed.
// One record per runOnce: a session's FSM leaves Established at most once.
type sessionLeave struct {
	signal     chan struct{}
	closed     sessionClose
	notified   bool
	transition FSMTransition
	bfd        bfdRelease
}

// sessionEndHooks are test seams on the session end. Each nil hook is skipped.
// Production never sets Peer.sessionEndHooks.
type sessionEndHooks struct {
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

	// The close is derived once, from the event that made this
	// transition, from whether a NOTIFICATION crossed the wire, and
	// from the close reason the session recorded, and every consumer
	// reads it: Graceful Restart and BMP both decide on it
	// (peer_close.go).
	leave.notified = p.notificationExchanged.Load()
	leave.closed = sessionCloseFor(leave.notified, event, closeCause(session))

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
		Reason:    leave.closed.reason,
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
//  4. It raises session-dropped when no NOTIFICATION was exchanged, tells the
//     plugins down, logs "session closed" and appends the history row.
//
// So a plugin hears every UPDATE of a session before its down, and the down
// follows PeerStateConnecting.
func (p *Peer) sessionDown(session *Session, deliverChan chan deliveryItem, deliveryDone <-chan struct{}, leave *sessionLeave) {
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

	reason := leave.closed.reason
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
		p.announceDown(reactor, leave.closed)
	}
	peerLogger().Info("session closed", "peer", p.settings.Address.String(), "reason", reason)
	p.history.append(leave.transition)
}

// failReceivePublication records that the delivery worker could not publish a
// batch, and on the first failure tears the session down: its routes would
// otherwise stay installed while its UPDATEs never reach the plugins.
// It MUST NOT wait for the delivery worker, which calls it:
// Session.teardownAutomatic sends the NOTIFICATION, closes the connection and
// signals Run, and waits on neither.
func failReceivePublication(session *Session, publication *receivePublication) {
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
	if err := session.teardownAutomatic(message.NotifyCeaseOutOfResources, ""); err != nil {
		peerLogger().Warn("receive publication failed, teardown failed",
			"peer", session.settings.Address, "error", err)
	}
}
