// Design: docs/architecture/behavior/fsm.md — optional RFC 4271 DelayOpen timer.
package fsm

import "time"

// SetDelayOpenTime sets the configured delay. The owner MUST call this before
// starting the session; zero preserves immediate OPEN transmission.
func (t *Timers) SetDelayOpenTime(d time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.delayOpenTime = d
}

// OnDelayOpenTimerExpires installs the session-owned expiry callback before start.
func (t *Timers) OnDelayOpenTimerExpires(cb TimerCallback) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.onDelayOpenExpires = cb
}

// StartDelayOpenTimer arms the configured delay and reports whether it is
// enabled. A retired set (Retire) arms nothing and answers true, so the caller
// holds the OPEN of a session that is over rather than send it.
// RFC 4271 Section 8.2.2: "sets the DelayOpenTimer to the initial value".
// The session MUST stop it on completion or teardown, including a fired callback.
func (t *Timers) StartDelayOpenTimer() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.delayOpenTime == 0 {
		return false
	}
	t.stopDelayOpenTimerLocked()
	if t.retired {
		// The session is over: holding the OPEN is the answer that sends nothing.
		return true
	}
	t.delayOpenGen++
	gen := t.delayOpenGen
	t.delayOpenRunning = true
	t.delayOpenTimer = t.clock.AfterFunc(t.delayOpenTime, func() { t.fireDelayOpen(gen) })
	return true
}

func (t *Timers) fireDelayOpen(gen uint64) {
	t.mu.Lock()
	if gen != t.delayOpenGen {
		t.mu.Unlock()
		return
	}
	cb := t.onDelayOpenExpires
	t.mu.Unlock()
	// The session serializes expiry with receipt of OPEN. Keep the pending
	// timer visible until that winner consumes it, rather than creating an
	// OPEN-less interval in which Event 18 or collision sees no held socket.
	if cb != nil {
		cb()
	}
}

// StopDelayOpenTimer clears a running or fired timer. The owner MUST call it
// after StartDelayOpenTimer when either OPEN processing or teardown wins.
func (t *Timers) StopDelayOpenTimer() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.stopDelayOpenTimerLocked()
}

func (t *Timers) stopDelayOpenTimerLocked() {
	if t.delayOpenTimer != nil {
		t.delayOpenTimer.Stop()
		t.delayOpenTimer = nil
	}
	t.delayOpenGen++
	t.delayOpenRunning = false
}

// IsDelayOpenTimerRunning reports an OPEN wait not yet consumed by the session.
func (t *Timers) IsDelayOpenTimerRunning() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.delayOpenRunning
}
