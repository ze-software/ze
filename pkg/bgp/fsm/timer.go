package fsm

import (
	"sync"
	"time"
)

// Default timer values per RFC 4271.
const (
	DefaultHoldTime         = 90 * time.Second  // Default hold time
	DefaultConnectRetryTime = 120 * time.Second // Default connect retry time
)

// TimerCallback is called when a timer expires.
type TimerCallback func()

// Timers manages the BGP FSM timers per RFC 4271 Section 8.
//
// Three timers are used:
// - HoldTimer: Detects dead peers (default 90s, negotiated in OPEN).
// - KeepaliveTimer: Sends periodic KEEPALIVEs (hold_time/3).
// - ConnectRetryTimer: Delays between connection attempts (default 120s).
type Timers struct {
	mu sync.Mutex

	// Timer durations
	holdTime         time.Duration
	connectRetryTime time.Duration

	// Active timers
	holdTimer         *time.Timer
	keepaliveTimer    *time.Timer
	connectRetryTimer *time.Timer

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
		holdTime:         DefaultHoldTime,
		connectRetryTime: DefaultConnectRetryTime,
	}
}

// SetHoldTime sets the hold time duration.
// Keepalive timer will be hold_time/3.
// Setting to 0 disables both hold and keepalive timers.
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
func (t *Timers) StartHoldTimer() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.holdTime == 0 {
		return // Disabled
	}

	t.stopHoldTimerLocked()

	t.holdTimer = time.AfterFunc(t.holdTime, func() {
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
func (t *Timers) ResetHoldTimer() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.holdTime == 0 || !t.holdRunning {
		return
	}

	// Stop and restart
	t.stopHoldTimerLocked()

	t.holdTimer = time.AfterFunc(t.holdTime, func() {
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
			t.keepaliveTimer = time.AfterFunc(keepaliveInterval, timerFunc)
		}
		t.mu.Unlock()
	}

	t.keepaliveTimer = time.AfterFunc(keepaliveInterval, timerFunc)
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
func (t *Timers) StartConnectRetryTimer() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.stopConnectRetryTimerLocked()

	t.connectRetryTimer = time.AfterFunc(t.connectRetryTime, func() {
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
