// Design: docs/architecture/behavior/peer-lifecycle.md -- why a session left Established
// RFC: rfc/short/rfc4724.md
// RFC: rfc/short/rfc7854.md
// Related: peer_run.go -- the FSM callback that derives the close from its transition
// Related: reactor_api.go -- apiStateObserver, which hands the close to the plugins

package reactor

import (
	"errors"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// sessionClose says why an Established session closed, once, for every
// consumer of the down: the reason the plugins read (one of the rpc.Reason*
// constants) and the RFC 4271 Section 8.1 number of the FSM event that closed
// the session. fsmEvent is zero for a close no FSM event made, such as a peer
// removed from the configuration, which is the value RFC 7854 Section 4.9
// gives "no relevant Event code".
type sessionClose struct {
	reason   string
	fsmEvent uint16
}

// closeCause returns the first close reason recorded on session
// (Session.setCloseReason), or nil when nothing recorded one.
func closeCause(session *Session) error {
	cause := session.closeReason.Load()
	if cause == nil {
		return nil
	}
	return *cause
}

// sessionCloseFor derives the close of a session that left Established on
// event. notified is true when a NOTIFICATION crossed the wire on this session,
// in either direction (Peer.notificationExchanged). cause is the session's
// recorded close reason (closeCause), nil when none was recorded.
//
// The NOTIFICATION decides first, because two RFCs make it the deciding fact.
// RFC 4724 Section 4: "the normal BGP procedures MUST be followed when the
// TCP session terminates due to the sending or receiving of a BGP NOTIFICATION
// message." RFC 8538 relaxes that only when the "N" bit has been exchanged,
// and ze never advertises it (peer_gr_flags.go sets the R and F bits alone),
// so every NOTIFICATION here takes the normal procedures.
//
// Without a NOTIFICATION, the transport failing is its own cause. RFC 7854
// Section 4.9: "Reason 4: The remote system closed the session without a
// notification message.  This includes any unexpected termination of the
// transport session, so in some cases both the local and remote systems might
// consider this to apply." Event 18 is that termination.
//
// The GR-aware congestion teardown (forward_pool_congestion.go) also fires
// Event 18, so the GR state machine runs the procedure a transport loss runs,
// but nothing failed: ze closed the transport itself, with no NOTIFICATION.
// It records ErrCongestionTeardown as the session's close reason first, and
// that close is reported as a local close carrying Event 18. RFC 7854 Section
// 4.9: "Reason 2: The local system closed the session.  No notification
// message was sent." Graceful Restart still retains routes for it, because
// only a NOTIFICATION takes the normal procedures.
//
// Any other event is ze closing the session with no NOTIFICATION reaching the
// peer, typically a Cease that could not be written.
func sessionCloseFor(notified bool, event fsm.Event, cause error) sessionClose {
	// RFC 7854 Section 4.9: "Two bytes both set to 0 are used to indicate
	// that no relevant Event code is defined." An event with no registered
	// number gets that answer rather than ze's internal ordinal.
	number, registered := event.Number()
	if !registered {
		number = 0
	}

	if notified {
		return sessionClose{reason: rpc.ReasonNotification, fsmEvent: number}
	}
	if event != fsm.EventTCPConnectionFails {
		return sessionClose{reason: rpc.ReasonLocalClose, fsmEvent: number}
	}
	// RFC 7854 Section 4.9: "Reason 2: The local system closed the session.
	// No notification message was sent."
	if errors.Is(cause, ErrCongestionTeardown) {
		return sessionClose{reason: rpc.ReasonLocalClose, fsmEvent: number}
	}
	return sessionClose{reason: rpc.ReasonTCPFailure, fsmEvent: number}
}
