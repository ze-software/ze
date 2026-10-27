// Design: docs/architecture/behavior/fsm.md — RFC 4271 Section 8.2.2 DelayOpen.
package fsm

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
