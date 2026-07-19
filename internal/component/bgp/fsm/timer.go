// Design: docs/architecture/behavior/fsm.md — BGP finite state machine
// RFC: rfc/short/rfc4271.md — hold timer and keepalive timer (Section 8)

package fsm

import (
	"sync"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/core/clock"
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
	clock clock.Clock

	// Timer durations
	holdTime         time.Duration
	keepaliveTime    time.Duration // 0 = derive from holdTime/3 (RFC 4271 Section 10)
	connectRetryTime time.Duration

	// Active timers
	holdTimer         clock.Timer
	keepaliveTimer    clock.Timer
	connectRetryTimer clock.Timer

	// Callbacks
	onHoldExpires         TimerCallback
	onKeepaliveExpires    TimerCallback
	onConnectRetryExpires TimerCallback

	// State tracking
	holdRunning         bool
	keepaliveRunning    bool
	connectRetryRunning bool

	// Hold-timer generation guard. holdGen is bumped on every arm and on every
	// stop of a live hold timer, so a fired closure that captured an older
	// generation can detect that the timer was stopped or re-armed after it
	// fired and decline to touch shared state (the ABA seed described in the
	// fixit-bgp-session-fsm-lifecycle spec, A-2). holdFireGen records the
	// generation of the fire currently running its callback; GraceRearmHoldTimer
	// re-arms only while holdFireGen == holdGen, i.e. nothing (notably StopAll)
	// intervened during the callback (R-3).
	holdGen     uint64
	holdFireGen uint64
}

// NewTimers creates a new timer manager with default values.
func NewTimers() *Timers {
	return &Timers{
		clock:            clock.RealClock{},
		holdTime:         DefaultHoldTime,
		connectRetryTime: DefaultConnectRetryTime,
	}
}

// SetClock sets the clock used for timer operations.
// Must be called before starting any timers (typically via Session.SetClock).
func (t *Timers) SetClock(c clock.Clock) {
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

// SetKeepaliveTime sets an explicit keepalive interval.
// 0 means derive from holdTime/3 (RFC 4271 Section 10 default).
// Non-zero overrides the derivation. The FSM clamps this at negotiation
// time if the negotiated hold-time is smaller than the configured value.
func (t *Timers) SetKeepaliveTime(d time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.keepaliveTime = d
}

// KeepaliveTime returns the configured keepalive time (0 = auto).
func (t *Timers) KeepaliveTime() time.Duration {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.keepaliveTime
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

	t.armHoldTimerLocked(t.holdTime)
}

// armHoldTimerLocked (re)arms the hold timer for duration d. It is the single
// place a hold timer's AfterFunc is created; StartHoldTimer, ResetHoldTimer and
// GraceRearmHoldTimer all funnel through it so the generation guard and the
// fire path stay in one spot (collapsing the previously duplicated closures).
// The caller must hold t.mu.
func (t *Timers) armHoldTimerLocked(d time.Duration) {
	if d <= 0 {
		// Self-enforcing invariant: never schedule a non-positive AfterFunc.
		// All current callers already guard this (holdTime != 0, grace clamp),
		// but keeping the check here stops a future caller arming AfterFunc(0).
		t.stopHoldTimerLocked()
		return
	}
	t.stopHoldTimerLocked() // bumps holdGen if a timer was live
	t.holdGen++
	gen := t.holdGen
	t.holdTimer = t.clock.AfterFunc(d, func() { t.fireHold(gen) })
	t.holdRunning = true
}

// fireHold runs when the hold timer's AfterFunc fires. gen is the generation
// captured when the timer was armed. If holdGen has advanced since (a Stop or a
// re-arm happened after this timer fired but before this closure took the lock),
// this is a stale fired closure and must not touch shared state — otherwise it
// would clear holdRunning out from under a freshly armed timer (spec A-2).
func (t *Timers) fireHold(gen uint64) {
	t.mu.Lock()
	if t.holdGen != gen {
		t.mu.Unlock()
		return // stale: timer was stopped or re-armed after it fired
	}
	t.holdRunning = false
	t.holdFireGen = gen // marks the window in which GraceRearmHoldTimer may re-arm
	cb := t.onHoldExpires
	t.mu.Unlock()

	if cb != nil {
		cb()
	}
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

	t.armHoldTimerLocked(t.holdTime)
}

// GraceRearmHoldTimer re-arms the hold timer for a bounded grace window from
// within the hold-expiry callback, without requiring holdRunning (the fire path
// has already cleared it). This is the ONLY re-arm path that runs after a hold
// timer has expired; ordinary KEEPALIVE/UPDATE restarts go through
// ResetHoldTimer, whose !holdRunning guard deliberately keeps late FSM events
// from resurrecting a torn-down session's timer.
//
// It is generation-checked: it re-arms only if no Stop/arm intervened since the
// expiry that is currently running its callback (holdFireGen == holdGen). A
// racing StopAll therefore wins and the timer is not resurrected on a dead
// session (spec R-3). d is clamped to holdTime. holdTime == 0 stays disabled
// (RFC 4271 Section 4.4). Intended to be called only from the hold-expiry
// callback.
//
// The grace re-arm is a deliberate, documented divergence from RFC 4271
// Section 8.2.2 Event 10 (which mandates immediate teardown on HoldTimer expiry):
// it lets a session that saw recent read activity survive one expiry under CPU
// congestion, matching BIRD-style implementations, rather than dropping a peer
// that is merely slow. The next expiry with no intervening traffic still tears
// the session down.
func (t *Timers) GraceRearmHoldTimer(d time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.holdTime == 0 {
		return // RFC 4271 Section 4.4: hold time 0 disables the timer
	}
	// holdFireGen is 0 until a real expiry fires (arm always bumps holdGen to
	// >= 1 before capturing it), so holdFireGen == 0 means GraceRearmHoldTimer
	// was called outside an expiry callback — refuse. A non-zero holdFireGen
	// that no longer equals holdGen means a Stop/arm (e.g. a concurrent StopAll)
	// intervened after this expiry fired — refuse, so the timer is not
	// resurrected on a torn-down session (spec R-3).
	if t.holdFireGen == 0 || t.holdFireGen != t.holdGen {
		return
	}
	if d > t.holdTime {
		d = t.holdTime // clamp: never extend beyond the negotiated hold time
	}
	if d <= 0 {
		// A non-positive grace window means "do not extend": leave the timer
		// disarmed (the fire already cleared holdRunning). Deliberate no-op, not
		// a re-arm. Unreachable with the fixed 10 s production caller.
		return
	}
	t.armHoldTimerLocked(d)
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
		// Bump the generation so any already-fired closure that has not yet
		// taken the lock sees a mismatch and declines to touch state, and so a
		// grace re-arm racing this stop is rejected (spec A-2, R-3). Stop()'s
		// fired/not-fired return is intentionally not consulted: the generation
		// guard makes that distinction unnecessary.
		t.holdGen++
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
//
// NOTE (spec fixit-bgp-session-fsm-lifecycle, A-6): unlike the hold timer, the
// keepalive timer does not carry the generation guard. Its self-rescheduling
// closure gates every re-arm on keepaliveRunning, and StopKeepaliveTimer /
// StopAll clear that flag under the lock, so a stop always halts the chain
// (correctness-safe). A stale fired closure from a just-stopped-and-restarted
// chain could at worst schedule one extra keepalive (wire noise), never a
// correctness bug, so the guard is intentionally not extended here.
func (t *Timers) StartKeepaliveTimer() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.holdTime == 0 {
		return // Disabled when hold time is 0
	}

	t.stopKeepaliveTimerLocked()

	keepaliveInterval := t.holdTime / 3
	if t.keepaliveTime > 0 {
		keepaliveInterval = t.keepaliveTime
	}

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
