// Design: docs/architecture/behavior/fsm.md — RFC 4271 Section 8.2.2 DelayOpen.
package fsm

import "fmt"

// EnterDelayOpenBfdUpPending consumes Event 20's strict-mode branch after the
// session has sent OPEN. Validation and consumption of the running DelayOpen
// timer are atomic with the sub-state write; this cannot enter from OpenSent or
// re-enter after the peer's first OPEN. EnterBfdUpPending remains OpenSent-only.
func (f *FSM) EnterDelayOpenBfdUpPending() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if (f.state != StateConnect && f.state != StateActive) ||
		!f.bfdStrict || f.bfdSub != SubStateNone ||
		f.timers == nil || !f.timers.IsDelayOpenTimerRunning() {
		return fmt.Errorf("%w: DelayOpen BFD pending requested in %s", ErrFSMError, f.state)
	}

	// draft-ietf-idr-bgp-bfd-strict-mode Sections 8.3.5 and 8.4.5:
	// "stops the ConnectRetryTimer (if running) and sets the
	// ConnectRetryTimer to zero", "stops and clears the DelayOpenTimer
	// (sets the value to zero)".
	f.event = EventBGPOpenWithDelayOpenTimer
	f.timers.StopConnectRetryTimer()
	f.timers.StopDelayOpenTimer()
	f.timers.StopHoldTimer()
	f.timers.StopKeepaliveTimer()
	// "If the HoldTimer initial value is non-zero, resets the
	// HoldTimer to the negotiated value"; otherwise "resets the
	// HoldTimer value to zero" and "starts the BfdHoldTimer with the
	// value BfdHoldTime". In both branches no KEEPALIVE is sent.
	if f.timers.HoldTime() == 0 {
		f.timers.StartBfdHoldTimer()
	} else {
		f.timers.StartHoldTimer()
	}
	f.bfdSub = SubStateConnectDelayOpenBfdUpPending
	if f.state == StateActive {
		f.bfdSub = SubStateActiveDelayOpenBfdUpPending
	}
	return nil
}

// releaseDelayOpenBfdPending runs with f.mu held, after the session sends the
// withheld KEEPALIVE. Sections 8.3.1 and 8.4.1: "starts the KeepaliveTimer with
// the initial value" and "changes its state to OpenConfirm". Zero HoldTime
// leaves the KeepaliveTimer disabled; the BFD wait bound ends on either path.
func (f *FSM) releaseDelayOpenBfdPending() {
	f.timers.StopBfdHoldTimer()
	f.timers.StartKeepaliveTimer()
	f.change(StateOpenConfirm)
}

// handleDelayOpen handles the shared Connect/Active DelayOpen actions with f.mu
// held. Wire I/O remains session-owned. A false result leaves the event to the
// state's ordinary handler, preserving the disabled path and error handling.
func (f *FSM) handleDelayOpen(event Event) bool {
	if f.timers == nil {
		return false
	}
	//exhaustive:ignore // Only the shared DelayOpen branches; the caller dispatches every other event.
	switch event {
	case EventTCPConnectionConfirmed:
		// RFC 4271 Section 8.2.2: "If the DelayOpen attribute is set to TRUE, the local system:
		// - stops the ConnectRetryTimer (if running) and sets the ConnectRetryTimer to zero,
		// - sets the DelayOpenTimer to the initial value, and
		// - stays in the Connect state."
		f.timers.StopConnectRetryTimer()
		return f.timers.StartDelayOpenTimer()
	case EventDelayOpenTimerExpires:
		if !f.timers.IsDelayOpenTimerRunning() {
			return false
		}
		// RFC 4271 Section 8.2.2: "If the local system receives a DelayOpenTimer_Expires event (Event
		// 12), the local system:
		// - sets the ConnectRetryTimer to zero,
		// - stops and clears the DelayOpenTimer (set to zero),
		// - completes the BGP initialization,
		// - sends the OPEN message to its remote peer,
		// - sets its hold timer to a large value, and
		// - changes its state to OpenSent."
		// The session sets that large value and sends OPEN before this event.
		f.timers.StopConnectRetryTimer()
		f.timers.StopDelayOpenTimer()
		f.change(StateOpenSent)
		return true
	case EventBGPOpenWithDelayOpenTimer:
		if !f.timers.IsDelayOpenTimerRunning() {
			return false
		}
		// RFC 4271 Section 8.2.2: "If an OPEN message is received while the DelayOpenTimer is running
		// (Event 20), the local system:
		// - stops the ConnectRetryTimer (if running) and sets the ConnectRetryTimer to zero,
		// - completes the BGP initialization,
		// - stops and clears the DelayOpenTimer (sets the value to zero),
		// - sends an OPEN message,
		// - sends a KEEPALIVE message,
		// - if the HoldTimer initial value is non-zero,
		//   - starts the KeepaliveTimer with the initial value and
		//   - resets the HoldTimer to the negotiated value,
		//   else, if the HoldTimer initial value is zero,
		//   - resets the KeepaliveTimer and
		//   - resets the HoldTimer value to zero,
		// - and changes its state to OpenConfirm."
		// The session owns the writes.
		f.timers.StopConnectRetryTimer()
		f.timers.StopDelayOpenTimer()
		f.timers.StopHoldTimer()
		f.timers.StopKeepaliveTimer()
		f.timers.StartHoldTimer()
		f.timers.StartKeepaliveTimer()
		f.change(StateOpenConfirm)
		return true
	case EventNotifMsgVerErr:
		if !f.timers.IsDelayOpenTimerRunning() {
			return false
		}
		// RFC 4271 Section 8.2.2: "If the DelayOpenTimer is running, the local system:
		// - stops the ConnectRetryTimer (if running) and sets the ConnectRetryTimer to zero,
		// - stops and resets the DelayOpenTimer (sets to zero),
		// - releases all BGP resources,
		// - drops the TCP connection, and
		// - changes its state to Idle."
		// This running-timer clause has no ConnectRetryCounter increment.
		f.change(StateIdle)
		return true
	}
	return false
}
