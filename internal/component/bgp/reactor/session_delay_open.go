// Design: docs/architecture/behavior/fsm.md — RFC 4271 DelayOpen wire lifecycle.
package reactor

import (
	"time"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/core/env"
)

// delayOpenExpired owns Event 12's wire action. openMu orders it against OPEN
// receipt and collision retirement, so a canceled callback cannot send a second
// OPEN or resurrect a closed session.
func (s *Session) delayOpenExpired() {
	s.openMu.Lock()
	defer s.unlockOpen()
	if !s.holdsOpen() {
		return
	}
	if s.tearingDown.Load() {
		return
	}
	conn := s.Conn()
	if conn == nil {
		return
	}
	// RFC 4271 Section 8.2.2: "If the DelayOpenTimer_Expires event (Event 12) occurs in the
	// Connect state, the local system:
	// - sends an OPEN message to its peer,
	// - sets the HoldTimer to a large value, and
	// - changes its state to OpenSent."
	if err := s.sendOpen(conn); err != nil {
		s.setCloseReason(err)
		s.handleConnectionCloseLocked()
		return
	}
	s.timers.SetHoldTime(env.GetDuration("ze.bgp.openwait", 120*time.Second))
	s.timers.StartHoldTimer()
	s.logFSMEvent(fsm.EventDelayOpenTimerExpires)
}

// holdsOpen reports a connected DelayOpen wait, not a configured preference.
// A caller closing the connection MUST recheck under openMu; the accept gate
// only uses this snapshot to select the bounded pending-connection slot.
func (s *Session) holdsOpen() bool {
	state := s.fsm.State()
	if state != fsm.StateConnect && state != fsm.StateActive {
		return false
	}
	return s.timers.IsDelayOpenTimerRunning()
}

// closeHeldForCollision retires an OPEN-less connection when a competing OPEN
// arrives. RFC 4271 Section 8.2.2 Event 23 sends no NOTIFICATION in Connect or
// Active. A false answer means the caller MUST re-run Section 6.8 arbitration.
func (s *Session) closeHeldForCollision() bool {
	s.openMu.Lock()
	defer s.unlockOpen()
	if !s.holdsOpen() {
		return false
	}
	s.closeCollision(0, 0, false)
	return true
}

// retireSessionTimers is Run's exit timer stop. Run MUST call it on exit. It
// never waits for openMu (see handleConnectionClose): with the lock held, the
// stop is owed and the holder runs it as it releases, after its action list,
// so the action list cannot rearm a timer behind it. Transport closure alone
// preserves the DelayOpen flag until the failure owner delivers Event 18.
func (s *Session) retireSessionTimers() {
	s.retireOwed.Store(true)
	s.runOwedOpenWork()
}

// stopSessionTimers joins any in-flight initial OPEN work before the final
// timer stop; a callback cannot rearm HoldTimer after this finalizer returns.
// The Peer that owns the session MUST call it only after the negotiated
// publication is joined (endNegotiatedSession): that publication can hold
// openMu until it returns.
func (s *Session) stopSessionTimers() {
	s.openMu.Lock()
	defer s.unlockOpen()
	s.timers.StopAll()
}
