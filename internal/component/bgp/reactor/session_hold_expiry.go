// Design: docs/architecture/behavior/fsm-open-sent.md -- original HoldTimer expiry.
// Related: session_open_admission.go -- admission cannot replace an expired timer.
// RFC 4271 Sections 8.1.3 and 8.2.2 -- see rfc/short/rfc4271.md.
package reactor

import (
	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
)

// rejectExpiredHold MUST run under openMu before OPEN advancement, candidate
// collision publication or selection. HoldBudget owns the original deadline;
// a queued timer callback cannot grant time that already elapsed.
// RFC 4271 Section 8.1.3: "An event generated when the HoldTimer expires."
func (s *Session) rejectExpiredHold() bool {
	if s.tearingDown.Load() {
		return false
	}
	if s.timers.HoldBudget().State != fsm.HoldBudgetExpired {
		return false
	}
	// RFC 4271 Section 8.2.2, Event10.
	s.holdTimerExpiredLocked()
	return true
}

// holdTimerExpiredLocked is the single Event10 action list. Its timer callback
// and an admission owner MUST hold openMu; neither caller may hold Peer.mu.
// The terminal seal suppresses a queued callback after an admission already
// delivered expiry. The bounded NOTIFICATION precedes the cancellation signal,
// so cancellation cannot close its socket before the expiry reason is sent.
// RFC 4271 Section 8.1.3: "An event generated when the HoldTimer expires."
// Section 8.2.2 supplies its state's notification, counter and Idle actions.
func (s *Session) holdTimerExpiredLocked() {
	if s.tearingDown.Load() {
		return
	}
	s.setCloseReason(ErrHoldTimerExpired)
	s.seal()
	// The strict-mode diagnostic must inspect its timer/sub-state before the
	// terminal writer retires those timers and Event10 clears the sub-state.
	// draft-ietf-idr-bgp-bfd-strict-mode Section 10: "A BGP speaker SHOULD
	// log a message if it closes its session due to hold timer expiration
	// while waiting for the BFD hold-down interval."
	switch {
	case s.timers.IsBfdHoldDownTimerRunning():
		s.logHoldDownExpiry()
	case s.bfdHoldDownKeepalive.Load():
		s.logHoldDownExpiry()
	case s.fsm.BfdSubState() != fsm.SubStateNone:
		sessionLogger().Warn("bfd strict-mode: the BGP hold timer expired while waiting for BFD",
			"peer", s.settings.Address,
			"bfd-sub-state", s.fsm.BfdSubState().String(),
			"hold-time", s.timers.HoldTime(),
			"code", codeBFDWaitHoldExpired, "effect", "this BGP connection ends",
			"fix", bfdWaitHoldExpiryFix)
		s.reportBFDWaitHoldExpiry("waiting for BFD")
	}
	conn := s.Conn()
	if conn != nil {
		if err := s.sendNotificationWithin(conn, message.NotifyHoldTimerExpired, 0, nil,
			holdExpiryNotifyDeadline); err != nil {
			s.logNotifyOutcome(message.NotifyHoldTimerExpired, 0, err)
		}
	}
	// RFC 4271 Section 8.2.2: Event10 "increments the ConnectRetryCounter"
	// and "changes its state to Idle." FSM.Event owns the state/counter;
	// no Session mutex spans the callback. Run's cancellation/read finalizers
	// still own socket closure and resource cleanup after this signal.
	s.logFSMEvent(fsm.EventHoldTimerExpires)
	s.stopAdmission()
	select {
	case s.errChan <- ErrHoldTimerExpired:
	default: // The cancellation owner already holds a terminal signal.
	}
}

// logHoldDownExpiry is shared by the two real hold-down wait shapes.
// draft-ietf-idr-bgp-bfd-strict-mode Section 10: "A BGP speaker SHOULD
// log a message if it closes its session due to hold timer expiration
// while waiting for the BFD hold-down interval."
func (s *Session) logHoldDownExpiry() {
	sessionLogger().Warn("bfd strict-mode: the BGP hold timer expired while observing the BFD hold-down interval",
		"peer", s.settings.Address,
		"hold-down", s.timers.BfdHoldDown(),
		"hold-time", s.timers.HoldTime(),
		"code", codeBFDWaitHoldExpired, "effect", "this BGP connection ends",
		"fix", bfdWaitHoldExpiryFix)
	s.reportBFDWaitHoldExpiry("observing the BFD hold-down interval")
}
