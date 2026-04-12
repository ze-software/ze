// Design: rfc/short/rfc5880.md -- Detection time and TX timer (Section 6.8.4, 6.8.7)
// Related: session.go -- Machine state and identity
// Related: fsm.go -- reception procedure that drives the timers
//
// Detection-time arithmetic and periodic-TX deadline management.
//
// All time math runs in microseconds because RFC 5880 expresses every
// interval in microseconds. Care is taken to use monotonic time (clock.Clock
// returns time.Time, which carries a monotonic component) so wall-clock
// jumps do not produce false detection events.
package session

import (
	"time"

	"codeberg.org/thomas-mangin/ze/internal/plugins/bfd/packet"
)

// DetectionInterval returns the current detection time as a Go duration.
//
// RFC 5880 Section 6.8.4 (Asynchronous mode):
//
//	detect_time = remote_detect_mult * max(local_RequiredMinRx, remote_DesiredMinTx)
//
// We approximate the "remote DesiredMinTx" with the most recently received
// value, which is captured implicitly: bfd.RemoteMinRxInterval is the floor
// the peer requires us to honor, and is updated on every received packet.
// In strict-conformance mode the spec uses the peer's advertised TX rate;
// the engine stores both fields and the larger is used.
//
// While the session is not Up, the floor is the slow-start interval.
func (m *Machine) DetectionInterval() time.Duration {
	mult := uint32(m.vars.RemoteDetectMult)
	if mult == 0 {
		mult = uint32(m.vars.DetectMult)
	}
	floor := max(m.vars.RequiredMinRxInterval, m.vars.RemoteMinRxInterval)
	if floor == 0 {
		floor = SlowStartIntervalUs
	}
	usec := uint64(mult) * uint64(floor)
	return time.Duration(usec) * time.Microsecond
}

// TransmitInterval returns the current TX inter-packet interval as a Go
// duration. RFC 5880 Section 6.8.7:
//
//	tx_interval = max(bfd.DesiredMinTxInterval, bfd.RemoteMinRxInterval)
//
// Jitter is applied per-packet by the engine, not here.
func (m *Machine) TransmitInterval() time.Duration {
	tx := max(m.vars.DesiredMinTxInterval, m.vars.RemoteMinRxInterval)
	if tx == 0 {
		tx = SlowStartIntervalUs
	}
	return time.Duration(tx) * time.Microsecond
}

// armDetectionLocked sets the next-detection deadline relative to now. The
// caller MUST hold the implicit single-owner lock (i.e., be the express
// loop goroutine).
func (m *Machine) armDetectionLocked(now time.Time) {
	m.nextDetectAt = now.Add(m.DetectionInterval())
}

// CheckDetection runs the detection-timer expiry check. Call from the
// express loop whenever now is at or past nextDetectAt. If the timer has
// expired and the session is in Init or Up, the FSM transitions to Down
// with diagnostic 1 (Control Detection Time Expired).
//
// Returns true if a state change occurred. The notify callback fires
// before CheckDetection returns.
func (m *Machine) CheckDetection(now time.Time) bool {
	if m.nextDetectAt.IsZero() {
		return false
	}
	if now.Before(m.nextDetectAt) {
		return false
	}
	if m.vars.SessionState != packet.StateInit && m.vars.SessionState != packet.StateUp {
		return false
	}
	prev := m.vars.SessionState
	m.vars.LocalDiag = packet.DiagControlDetectExpired
	m.vars.SessionState = packet.StateDown
	// Clear the detection deadline so subsequent ticks do not see a
	// stale past time. RFC 5880 §6.8.1 also clears bfd.RemoteDiscr on
	// detection-time expiry; that is handled in onStateChange via the
	// "entry to Down" branch.
	m.nextDetectAt = time.Time{}
	m.onStateChange(prev)
	return true
}

// NextTxDeadline returns the time at which the next periodic Control
// packet should be transmitted, or zero if no periodic TX is currently
// scheduled (passive role waiting for first packet).
func (m *Machine) NextTxDeadline() time.Time { return m.nextTxAt }

// AdvanceTx records that a periodic TX just happened at now. The next-TX
// deadline moves forward by TransmitInterval().
//
// RFC 5880 Section 6.8.7 jitter is applied by the caller. AdvanceTx is
// jitter-free; the engine adds jitter when it schedules the next fire.
func (m *Machine) AdvanceTx(now time.Time) {
	m.nextTxAt = now.Add(m.TransmitInterval())
}

// AdvanceTxWithJitter records a periodic TX and sets the next-TX deadline
// with an RFC 5880 Section 6.8.7 jitter reduction applied. The engine
// computes the reduction via Loop.applyJitter and passes it in.
//
// The function defensively clamps the reduction into [0, TransmitInterval)
// so a caller bug cannot drive nextTxAt backwards -- a backwards deadline
// would spin the express loop firing TX on every tick until something
// else advanced the clock. The only live caller is jitter-bounded to
// 25% of base and is safe, but the clamp makes the contract mechanical.
func (m *Machine) AdvanceTxWithJitter(now time.Time, reduction time.Duration) {
	interval := m.TransmitInterval()
	if reduction < 0 || reduction >= interval {
		reduction = 0
	}
	m.nextTxAt = now.Add(interval - reduction)
}

// LastReceived returns the timestamp of the most recently accepted Control
// packet, or zero if none has been received.
func (m *Machine) LastReceived() time.Time { return m.lastRxTime }

// PollOutstanding reports whether the session is currently sending Poll
// packets and waiting for an F-bit reply.
func (m *Machine) PollOutstanding() bool { return m.vars.PollOutstanding }

// DesiredMinTxIntervalUs returns the live bfd.DesiredMinTxInterval in
// microseconds. Exposed so the engine and tests can observe timer
// negotiation without reaching into the unexported Vars struct.
func (m *Machine) DesiredMinTxIntervalUs() uint32 { return m.vars.DesiredMinTxInterval }

// EchoEnabled reports whether the session has echo mode configured
// locally AND the peer has advertised a non-zero
// RequiredMinEchoRxInterval. Stage 6 uses this to gate the engine's
// per-session echo scheduler; without both ends opting in, no echo
// packets flow.
func (m *Machine) EchoEnabled() bool {
	return m.vars.DesiredMinEchoTxInterval != 0 &&
		m.vars.RemoteMinEchoRxInterval != 0
}

// EchoInterval returns the negotiated echo TX cadence:
// max(local DesiredMinEchoTx, peer RemoteMinEchoRx), as a Go
// duration. Returns zero when echo is not active.
func (m *Machine) EchoInterval() time.Duration {
	if !m.EchoEnabled() {
		return 0
	}
	us := uint64(m.vars.DesiredMinEchoTxInterval)
	if r := uint64(m.vars.RemoteMinEchoRxInterval); r > us {
		us = r
	}
	return time.Duration(us) * time.Microsecond
}

// NextEchoTxDeadline returns the time at which the next echo
// packet should be transmitted, or zero when echo is not currently
// scheduled. Caller is the engine express-loop; the deadline is
// initialized on the first echoTick and advanced by AdvanceEcho.
func (m *Machine) NextEchoTxDeadline() time.Time { return m.nextEchoAt }

// AdvanceEcho records that an echo packet was transmitted at now and
// sets the next-echo deadline by EchoInterval. The engine calls this
// right after the transport accepts the outbound packet.
func (m *Machine) AdvanceEcho(now time.Time) {
	interval := m.EchoInterval()
	if interval <= 0 {
		m.nextEchoAt = time.Time{}
		return
	}
	m.nextEchoAt = now.Add(interval)
}

// PrimeEcho arms the echo timer for the first-ever TX at now.
// Idempotent: if the timer is already armed the call is a no-op so
// an echo that fires on the same tick does not reset its own
// schedule. The engine calls PrimeEcho from echoTick whenever the
// session is Up and echo is enabled.
func (m *Machine) PrimeEcho(now time.Time) {
	if !m.EchoEnabled() {
		m.nextEchoAt = time.Time{}
		return
	}
	if m.nextEchoAt.IsZero() {
		m.nextEchoAt = now
	}
}

// NextEchoSequence returns the next monotonic echo sequence number
// and advances the counter. Wraps cleanly around uint32.
func (m *Machine) NextEchoSequence() uint32 {
	m.echoSequence++
	return m.echoSequence
}

// LastEchoRTT returns the most recent echo round-trip observation.
// Zero until the first reflected echo is matched.
func (m *Machine) LastEchoRTT() time.Duration { return m.lastEchoRTT }

// RecordEchoRTT stores a reflected echo round-trip time. Called from
// the engine echo RX handler on every matched return packet.
func (m *Machine) RecordEchoRTT(rtt time.Duration) { m.lastEchoRTT = rtt }

// ClearEchoSchedule resets the echo timer. Called when a session
// leaves the Up state so stale deadlines do not fire echoes while
// the control path is still tearing down.
func (m *Machine) ClearEchoSchedule() {
	m.nextEchoAt = time.Time{}
}
