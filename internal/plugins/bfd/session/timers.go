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
