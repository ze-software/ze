// Design: docs/architecture/behavior/fsm.md — BGP finite state machine
// RFC: rfc/short/rfc4271.md — hold timer and keepalive timer (Section 8)

package fsm

import (
	"sync"
	"time"

	"github.com/ze-software/ze/internal/core/clock"
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

	// DefaultBfdHoldTime is draft-ietf-idr-bgp-bfd-strict-mode Section 3,
	// attribute 18 (BfdHoldTime): "The default value for this attribute is 30
	// seconds and is user configurable." The peer's bfd hold-time leaf is that
	// configuration (reactor/config.go, parseBFDSettings).
	DefaultBfdHoldTime = 30 * time.Second

	// DefaultBfdHoldDown is the BFD hold-down interval of
	// draft-ietf-idr-bgp-bfd-strict-mode Section 10, and it is ZERO by
	// deliberate choice.
	//
	// The draft gives the interval no default. It calls it "the locally
	// configured BFD hold-down interval", and Section 10 offers it as a
	// mechanism that "may help reduce the frequency of BGP session flaps"
	// rather than one every session runs. Zero means the session advances on
	// the first BFD Up, which is what a peer with no hold-down leaf did before
	// the interval existed, so an operator who does not ask for damping is not
	// given any.
	DefaultBfdHoldDown = time.Duration(0)
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
	bfdHoldTime      time.Duration // draft-ietf-idr-bgp-bfd-strict-mode Section 3, attribute 18
	bfdHoldDown      time.Duration // draft-ietf-idr-bgp-bfd-strict-mode Section 10

	// Active timers
	holdTimer         clock.Timer
	keepaliveTimer    clock.Timer
	connectRetryTimer clock.Timer
	bfdHoldTimer      clock.Timer
	bfdHoldDownTimer  clock.Timer

	// Callbacks
	onHoldExpires         TimerCallback
	onKeepaliveExpires    TimerCallback
	onConnectRetryExpires TimerCallback
	onBfdHoldExpires      TimerCallback
	onBfdHoldDownExpires  TimerCallback

	// State tracking
	holdRunning         bool
	keepaliveRunning    bool
	connectRetryRunning bool
	bfdHoldRunning      bool
	bfdHoldDownRunning  bool

	// BfdHoldTimer generation guard, the same ABA defense the hold timer
	// carries below: a fired closure that captured an older generation
	// declines to touch shared state.
	bfdHoldGen uint64

	// BFD hold-down generation guard, same contract as bfdHoldGen.
	bfdHoldDownGen uint64

	// Hold-timer generation guard. holdGen is bumped on every arm and on every
	// stop of a live hold timer, so a fired closure that captured an older
	// generation can detect that the timer was stopped or re-armed after it
	// fired and decline to touch shared state (the ABA seed described in the
	// fixit-bgp-session-fsm-lifecycle spec, A-2).
	holdGen uint64
}

// NewTimers creates a new timer manager with default values.
func NewTimers() *Timers {
	return &Timers{
		clock:            clock.RealClock{},
		holdTime:         DefaultHoldTime,
		connectRetryTime: DefaultConnectRetryTime,
		bfdHoldTime:      DefaultBfdHoldTime,
		bfdHoldDown:      DefaultBfdHoldDown,
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
// place a hold timer's AfterFunc is created; StartHoldTimer and ResetHoldTimer
// both funnel through it so the generation guard and the fire path stay in one
// spot (collapsing the previously duplicated closures). Those two are the ONLY
// re-arm paths: nothing re-arms after an expiry, because RFC 4271 Section 8.2.2
// Event 10 tears the session down on the first one. The caller must hold t.mu.
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
	t.stopBfdHoldTimerLocked()
	t.stopBfdHoldDownTimerLocked()
}

// SetBfdHoldDown sets the BFD hold-down interval of
// draft-ietf-idr-bgp-bfd-strict-mode Section 10. Zero disables the wait, which
// is the behavior of a peer that configures no hold-down.
//
// Zero is NOT clamped to a default here, unlike SetBfdHoldTime above, and the
// difference is the draft's: BfdHoldTime is a session attribute with a stated
// default of 30 seconds, while the hold-down interval is "locally configured"
// and the draft names no value for it.
func (t *Timers) SetBfdHoldDown(d time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if d < 0 {
		d = 0
	}
	t.bfdHoldDown = d
}

// BfdHoldDown returns the configured BFD hold-down interval.
func (t *Timers) BfdHoldDown() time.Duration {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.bfdHoldDown
}

// OnBfdHoldDownTimerExpires registers the callback that runs when the BFD
// session has been Up for the whole hold-down interval.
func (t *Timers) OnBfdHoldDownTimerExpires(cb TimerCallback) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.onBfdHoldDownExpires = cb
}

// StartBfdHoldDownTimer arms the hold-down timer for BfdHoldDown, and reports
// whether it armed one. It answers false for a zero interval, which is how the
// caller learns there is nothing to wait for.
//
// draft-ietf-idr-bgp-bfd-strict-mode Section 10: "If both the local and remote
// BGP speakers include the BFD Strict-Mode Capability, the BGP state machine is
// permitted to transition to the Established state from the OpenConfirm state
// after the locally configured BFD hold-down interval is observed. That is, the
// BFD session has been Up for the desired amount of time."
//
// Re-arming restarts the interval, because the sentence measures how long the
// session has been Up rather than how long ago it first came Up.
func (t *Timers) StartBfdHoldDownTimer() bool {
	t.mu.Lock()
	interval := t.bfdHoldDown
	t.mu.Unlock()
	return t.StartBfdHoldDownTimerFor(interval)
}

// StartBfdHoldDownTimerFor arms the hold-down timer for a stated duration, which
// is what a caller uses when part of the interval has already been served.
//
// draft-ietf-idr-bgp-bfd-strict-mode Section 10 measures how long "the BFD
// session has been Up", not how long ago the BGP session noticed, so a session
// that came Up before this connection attempt owes only the remainder. A
// non-positive duration arms nothing and answers false, so the caller advances
// rather than waiting on a timer that would never fire.
func (t *Timers) StartBfdHoldDownTimerFor(interval time.Duration) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.stopBfdHoldDownTimerLocked()
	if interval <= 0 {
		return false
	}
	t.bfdHoldDownGen++
	gen := t.bfdHoldDownGen
	t.bfdHoldDownTimer = t.clock.AfterFunc(interval, func() { t.fireBfdHoldDown(gen) })
	t.bfdHoldDownRunning = true
	return true
}

// fireBfdHoldDown runs when the hold-down timer's AfterFunc fires. A generation
// mismatch means the timer was stopped or re-armed after this closure fired, so
// it must not touch state and must not release the wait.
func (t *Timers) fireBfdHoldDown(gen uint64) {
	t.mu.Lock()
	if t.bfdHoldDownGen != gen {
		t.mu.Unlock()
		return
	}
	t.bfdHoldDownRunning = false
	cb := t.onBfdHoldDownExpires
	t.mu.Unlock()

	if cb != nil {
		cb()
	}
}

// StopBfdHoldDownTimer stops the hold-down timer. A BFD session that leaves the
// Up state before the interval elapses has not been Up for the desired amount
// of time, so the wait it started is abandoned rather than completed.
func (t *Timers) StopBfdHoldDownTimer() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.stopBfdHoldDownTimerLocked()
}

func (t *Timers) stopBfdHoldDownTimerLocked() {
	if t.bfdHoldDownTimer != nil {
		t.bfdHoldDownTimer.Stop()
		t.bfdHoldDownTimer = nil
		t.bfdHoldDownGen++
	}
	t.bfdHoldDownRunning = false
}

// IsBfdHoldDownTimerRunning reports whether the hold-down timer is running.
func (t *Timers) IsBfdHoldDownTimerRunning() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.bfdHoldDownRunning
}

// SetBfdHoldTime sets the BfdHoldTime attribute of
// draft-ietf-idr-bgp-bfd-strict-mode Section 3, item 18. Zero restores the
// draft's own default of 30 seconds, so a peer that configures no hold-time
// gets the value the draft names rather than a timer that never fires.
func (t *Timers) SetBfdHoldTime(d time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if d <= 0 {
		t.bfdHoldTime = DefaultBfdHoldTime
		return
	}
	t.bfdHoldTime = d
}

// BfdHoldTime returns the current BfdHoldTime.
func (t *Timers) BfdHoldTime() time.Duration {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.bfdHoldTime
}

// OnBfdHoldTimerExpires registers the callback that raises FSM Event 34,
// BfdHoldTimerExpires (draft-ietf-idr-bgp-bfd-strict-mode Section 4).
func (t *Timers) OnBfdHoldTimerExpires(cb TimerCallback) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.onBfdHoldExpires = cb
}

// StartBfdHoldTimer arms the BfdHoldTimer with BfdHoldTime.
//
// draft-ietf-idr-bgp-bfd-strict-mode Section 8.5.5 starts it in exactly one
// place: an OPEN received while the strict-mode wait is on AND "the HoldTimer
// negotiated value is zero". A negotiated hold time of zero means RFC 4271
// starts no HoldTimer, so without this timer a session waiting for BFD would
// wait forever.
func (t *Timers) StartBfdHoldTimer() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.stopBfdHoldTimerLocked()
	t.bfdHoldGen++
	gen := t.bfdHoldGen
	t.bfdHoldTimer = t.clock.AfterFunc(t.bfdHoldTime, func() { t.fireBfdHold(gen) })
	t.bfdHoldRunning = true
}

// fireBfdHold runs when the BfdHoldTimer's AfterFunc fires. gen is the
// generation captured when the timer was armed; a mismatch means the timer was
// stopped or re-armed after this closure fired, so it must not touch state.
func (t *Timers) fireBfdHold(gen uint64) {
	t.mu.Lock()
	if t.bfdHoldGen != gen {
		t.mu.Unlock()
		return
	}
	t.bfdHoldRunning = false
	cb := t.onBfdHoldExpires
	t.mu.Unlock()

	if cb != nil {
		cb()
	}
}

// StopBfdHoldTimer stops the BfdHoldTimer and sets it to zero.
//
// draft-ietf-idr-bgp-bfd-strict-mode Sections 8.3, 8.4 and 8.5: "The
// BfdHoldTimer is reset to zero and stopped on any transition to the Idle
// state." Sections 8.3.1, 8.4.1 and 8.5.1 also reset it when the wait ends.
func (t *Timers) StopBfdHoldTimer() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.stopBfdHoldTimerLocked()
}

func (t *Timers) stopBfdHoldTimerLocked() {
	if t.bfdHoldTimer != nil {
		t.bfdHoldTimer.Stop()
		t.bfdHoldTimer = nil
		t.bfdHoldGen++
	}
	t.bfdHoldRunning = false
}

// IsBfdHoldTimerRunning reports whether the BfdHoldTimer is running.
func (t *Timers) IsBfdHoldTimerRunning() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.bfdHoldRunning
}
