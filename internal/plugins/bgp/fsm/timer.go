package fsm

import (
	"sync"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/sim"
)

// Default timer values per RFC 4271 Section 10.
//
// RFC 4271 Section 10:
//
//	"ConnectRetryTime is a mandatory FSM attribute that stores the initial
//	 value for the ConnectRetryTimer. The suggested default value for the
//	 ConnectRetryTime is 120 seconds."
//
//	"HoldTime is a mandatory FSM attribute that stores the initial value
//	 for the HoldTimer. The suggested default value for the HoldTime is
//	 90 seconds."
const (
	DefaultHoldTime         = 90 * time.Second  // RFC 4271 Section 10: suggested default 90s
	DefaultConnectRetryTime = 120 * time.Second // RFC 4271 Section 10: suggested default 120s
)

// TimerCallback is called when a timer expires.
type TimerCallback func()

// Timers manages the BGP FSM timers per RFC 4271 Sections 8 and 10.
//
// RFC 4271 Section 10 defines five mandatory timers for BGP:
//   - ConnectRetryTimer (Section 8.1.3, Event 9)
//   - HoldTimer (Section 8.1.3, Event 10)
//   - KeepaliveTimer (Section 8.1.3, Event 11)
//   - MinASOriginationIntervalTimer (Section 9.2.1.2) - not implemented here
//   - MinRouteAdvertisementIntervalTimer (Section 9.2.1.1) - not implemented here
//
// Two optional timers (DelayOpenTimer, IdleHoldTimer) are described in
// Section 8.1.3 Events 12-13, but are not implemented.
//
// Timer behaviors:
//   - HoldTimer: Detects dead peers. Restarted on KEEPALIVE/UPDATE receipt
//     (Section 8.2.2 Established state). Value negotiated per Section 4.2.
//   - KeepaliveTimer: Triggers periodic KEEPALIVE transmission.
//     RFC 4271 Section 10: "suggested default is 1/3 of the HoldTime"
//   - ConnectRetryTimer: Delays between connection attempts.
//
// NOTE: RFC 4271 Section 10 SHOULD requirement not implemented:
//
//	"To minimize the likelihood that the distribution of BGP messages by a
//	 given BGP speaker will contain peaks, jitter SHOULD be applied to the
//	 timers associated with MinASOriginationIntervalTimer, KeepaliveTimer,
//	 MinRouteAdvertisementIntervalTimer, and ConnectRetryTimer."
type Timers struct {
	mu sync.Mutex

	// Clock for injectable time operations.
	clock sim.Clock

	// Timer durations
	holdTime         time.Duration
	connectRetryTime time.Duration

	// Active timers
	holdTimer         sim.Timer
	keepaliveTimer    sim.Timer
	connectRetryTimer sim.Timer

	// Callbacks
	onHoldExpires         TimerCallback
	onKeepaliveExpires    TimerCallback
	onConnectRetryExpires TimerCallback

	// State tracking
	holdRunning         bool
	keepaliveRunning    bool
	connectRetryRunning bool
}

// NewTimers creates a new timer manager with default values.
func NewTimers() *Timers {
	return &Timers{
		clock:            sim.RealClock{},
		holdTime:         DefaultHoldTime,
		connectRetryTime: DefaultConnectRetryTime,
	}
}

// SetClock sets the clock used for timer operations.
// Must be called before starting any timers (typically via Session.SetClock).
func (t *Timers) SetClock(c sim.Clock) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.clock = c
}

// SetHoldTime sets the hold time duration.
// Keepalive timer will be hold_time/3 per RFC 4271 Section 10.
// Setting to 0 disables both hold and keepalive timers per RFC 4271 Section 4.4:
//
//	"If the negotiated Hold Time interval is zero, then periodic KEEPALIVE
//	 messages MUST NOT be sent."
func (t *Timers) SetHoldTime(d time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.holdTime = d
}

// HoldTime returns the current hold time.
func (t *Timers) HoldTime() time.Duration {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.holdTime
}

// SetConnectRetryTime sets the connect retry timer duration.
func (t *Timers) SetConnectRetryTime(d time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.connectRetryTime = d
}

// OnHoldTimerExpires sets the callback for hold timer expiry.
func (t *Timers) OnHoldTimerExpires(cb TimerCallback) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.onHoldExpires = cb
}

// OnKeepaliveTimerExpires sets the callback for keepalive timer expiry.
func (t *Timers) OnKeepaliveTimerExpires(cb TimerCallback) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.onKeepaliveExpires = cb
}

// OnConnectRetryTimerExpires sets the callback for connect retry timer expiry.
func (t *Timers) OnConnectRetryTimerExpires(cb TimerCallback) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.onConnectRetryExpires = cb
}

// StartHoldTimer starts the hold timer.
// Does nothing if hold time is 0.
//
// RFC 4271 Section 8.2.2 (OpenSent state):
//
//	"sets the HoldTimer to a large value" (suggested 4 minutes per Section 10)
//
// RFC 4271 Section 8.2.2 (OpenConfirm/Established states):
//
//	"If the negotiated hold time value is zero, then the HoldTimer and
//	 KeepaliveTimer are not started."
func (t *Timers) StartHoldTimer() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.holdTime == 0 {
		return // Disabled
	}

	t.stopHoldTimerLocked()

	t.holdTimer = t.clock.AfterFunc(t.holdTime, func() {
		t.mu.Lock()
		t.holdRunning = false
		cb := t.onHoldExpires
		t.mu.Unlock()

		if cb != nil {
			cb()
		}
	})
	t.holdRunning = true
}

// ResetHoldTimer resets the hold timer to its full duration.
// Should be called when KEEPALIVE or UPDATE is received.
//
// RFC 4271 Section 8.2.2 (Established state):
//
//	"If the local system receives a KEEPALIVE message (Event 26), the
//	 local system:
//	   - restarts its HoldTimer, if the negotiated HoldTime value is
//	     non-zero"
//	"If the local system receives an UPDATE message (Event 27), the
//	 local system:
//	   - restarts its HoldTimer, if the negotiated HoldTime value is
//	     non-zero"
func (t *Timers) ResetHoldTimer() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.holdTime == 0 || !t.holdRunning {
		return
	}

	// Stop and restart
	t.stopHoldTimerLocked()

	t.holdTimer = t.clock.AfterFunc(t.holdTime, func() {
		t.mu.Lock()
		t.holdRunning = false
		cb := t.onHoldExpires
		t.mu.Unlock()

		if cb != nil {
			cb()
		}
	})
	t.holdRunning = true
}

// StopHoldTimer stops the hold timer.
func (t *Timers) StopHoldTimer() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.stopHoldTimerLocked()
}

func (t *Timers) stopHoldTimerLocked() {
	if t.holdTimer != nil {
		t.holdTimer.Stop()
		t.holdTimer = nil
	}
	t.holdRunning = false
}

// IsHoldTimerRunning returns true if the hold timer is running.
func (t *Timers) IsHoldTimerRunning() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.holdRunning
}

// StartKeepaliveTimer starts the keepalive timer (hold_time/3).
// Does nothing if hold time is 0.
//
// RFC 4271 Section 4.4:
//
//	"KEEPALIVE messages are exchanged between peers often enough not to
//	 cause the Hold Timer to expire. A reasonable maximum time between
//	 KEEPALIVE messages would be one third of the Hold Time interval."
//
// RFC 4271 Section 10:
//
//	"The KeepaliveTime is a mandatory FSM attribute that stores the
//	 initial value for the KeepaliveTimer. The suggested default value
//	 for the KeepaliveTime is 1/3 of the HoldTime."
//
// RFC 4271 Section 8.2.2 (Established state):
//
//	"Each time the local system sends a KEEPALIVE or UPDATE message, it
//	 restarts its KeepaliveTimer, unless the negotiated HoldTime value
//	 is zero."
func (t *Timers) StartKeepaliveTimer() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.holdTime == 0 {
		return // Disabled when hold time is 0
	}

	t.stopKeepaliveTimerLocked()

	keepaliveInterval := t.holdTime / 3

	var timerFunc func()
	timerFunc = func() {
		t.mu.Lock()
		cb := t.onKeepaliveExpires
		running := t.keepaliveRunning
		t.mu.Unlock()

		if cb != nil && running {
			cb()
		}

		// Reschedule for periodic firing
		t.mu.Lock()
		if t.keepaliveRunning {
			t.keepaliveTimer = t.clock.AfterFunc(keepaliveInterval, timerFunc)
		}
		t.mu.Unlock()
	}

	t.keepaliveTimer = t.clock.AfterFunc(keepaliveInterval, timerFunc)
	t.keepaliveRunning = true
}

// StopKeepaliveTimer stops the keepalive timer.
func (t *Timers) StopKeepaliveTimer() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.stopKeepaliveTimerLocked()
}

func (t *Timers) stopKeepaliveTimerLocked() {
	t.keepaliveRunning = false
	if t.keepaliveTimer != nil {
		t.keepaliveTimer.Stop()
		t.keepaliveTimer = nil
	}
}

// IsKeepaliveTimerRunning returns true if the keepalive timer is running.
func (t *Timers) IsKeepaliveTimerRunning() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.keepaliveRunning
}

// StartConnectRetryTimer starts the connect retry timer.
//
// RFC 4271 Section 8.1.3:
//
//	"Event 9: ConnectRetryTimer_Expires
//	 Definition: An event generated when the ConnectRetryTimer expires.
//	 Status: Mandatory"
//
// RFC 4271 Section 10:
//
//	"ConnectRetryTime is a mandatory FSM attribute that stores the initial
//	 value for the ConnectRetryTimer. The suggested default value for the
//	 ConnectRetryTime is 120 seconds."
func (t *Timers) StartConnectRetryTimer() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.stopConnectRetryTimerLocked()

	t.connectRetryTimer = t.clock.AfterFunc(t.connectRetryTime, func() {
		t.mu.Lock()
		t.connectRetryRunning = false
		cb := t.onConnectRetryExpires
		t.mu.Unlock()

		if cb != nil {
			cb()
		}
	})
	t.connectRetryRunning = true
}

// StopConnectRetryTimer stops the connect retry timer.
func (t *Timers) StopConnectRetryTimer() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.stopConnectRetryTimerLocked()
}

func (t *Timers) stopConnectRetryTimerLocked() {
	if t.connectRetryTimer != nil {
		t.connectRetryTimer.Stop()
		t.connectRetryTimer = nil
	}
	t.connectRetryRunning = false
}

// IsConnectRetryTimerRunning returns true if the connect retry timer is running.
func (t *Timers) IsConnectRetryTimerRunning() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.connectRetryRunning
}

// StopAll stops all timers.
func (t *Timers) StopAll() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.stopHoldTimerLocked()
	t.stopKeepaliveTimerLocked()
	t.stopConnectRetryTimerLocked()
}
