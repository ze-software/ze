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

// EchoSlowdownIntervalUs is the RFC 5880 §6.8.9 minimum interval
// floor applied to both DesiredMinTxInterval and RequiredMinRxInterval
// while echo is active. One second (1,000,000 µs) matches the RFC
// recommendation of not less than one second.
const EchoSlowdownIntervalUs uint32 = 1_000_000

// ClearEchoSchedule resets the echo timer, drops every outstanding
// ring entry, and reverts any active echo slow-down. Called when a
// session leaves the Up state so stale deadlines do not fire echoes
// while the control path is still tearing down, and so a session
// that flaps back up does not carry dead entries or a stale
// slow-down flag into the new detection window.
func (m *Machine) ClearEchoSchedule() {
	m.nextEchoAt = time.Time{}
	for i := range m.echoOutstanding {
		m.echoOutstanding[i] = echoEntry{}
	}
	m.revertEchoSlowdownLocked()
}

// ApplyEchoSlowdown raises DesiredMinTxInterval and
// RequiredMinRxInterval to max(1s, configured) and initiates a
// Poll sequence so the peer learns the slowed rate atomically
// (RFC 5880 §6.8.3). Idempotent: a second call while the
// slow-down is already applied is a no-op. Called from
// engine.echoTickLocked when the session is Up and echo is
// negotiated.
func (m *Machine) ApplyEchoSlowdown() {
	if m.echoSlowdownApplied {
		return
	}
	m.echoSlowdownApplied = true
	m.vars.DesiredMinTxInterval = max(EchoSlowdownIntervalUs, m.vars.ConfiguredDesiredMinTxInterval)
	m.vars.RequiredMinRxInterval = max(EchoSlowdownIntervalUs, m.vars.ConfiguredRequiredMinRxInterval)
	m.vars.PollOutstanding = true
}

// revertEchoSlowdownLocked restores the configured intervals and
// initiates a Poll if the slow-down was active. Safe to call when
// the slow-down is not applied (no-op).
func (m *Machine) revertEchoSlowdownLocked() {
	if !m.echoSlowdownApplied {
		return
	}
	m.echoSlowdownApplied = false
	m.vars.DesiredMinTxInterval = m.vars.ConfiguredDesiredMinTxInterval
	m.vars.RequiredMinRxInterval = m.vars.ConfiguredRequiredMinRxInterval
	if m.vars.SessionState == packet.StateUp {
		m.vars.PollOutstanding = true
	}
}

// RegisterEchoTx adds an outstanding echo TX entry to the ring.
// When the ring is full the oldest live slot is overwritten; an
// overwrite is equivalent to a dropped echo from the detection
// standpoint because the lost slot never had a chance to match.
//
// Caller is the engine express loop (after transport.Send returns
// success) and is therefore the single writer. No synchronization
// beyond the express-loop owning goroutine is required.
func (m *Machine) RegisterEchoTx(seq uint32, now time.Time) {
	slot := -1
	oldest := -1
	var oldestAt time.Time
	for i := range m.echoOutstanding {
		e := m.echoOutstanding[i]
		if e.sentAt.IsZero() {
			slot = i
			break
		}
		if oldest == -1 || e.sentAt.Before(oldestAt) {
			oldest = i
			oldestAt = e.sentAt
		}
	}
	if slot == -1 {
		slot = oldest
	}
	m.echoOutstanding[slot] = echoEntry{sequence: seq, sentAt: now}
}

// MatchEchoRx scans the outstanding ring for a returning echo with
// the given sequence number. On match the slot is cleared and the
// observed round-trip time (now - sentAt) is returned with ok=true.
// An unmatched sequence returns (0, false) and leaves the ring
// untouched; the engine falls back to the self-carried ZEEC
// TimestampMs for RTT in that case.
//
// Caller MUST be the express-loop goroutine.
func (m *Machine) MatchEchoRx(seq uint32, now time.Time) (time.Duration, bool) {
	for i := range m.echoOutstanding {
		e := m.echoOutstanding[i]
		if e.sentAt.IsZero() || e.sequence != seq {
			continue
		}
		m.echoOutstanding[i] = echoEntry{}
		return now.Sub(e.sentAt), true
	}
	return 0, false
}

// EchoDetectInterval returns the echo-mode detection time, that is
// the maximum permitted silence between consecutive reflected
// echoes before the session is declared Down. The formula follows
// RFC 5880 Section 6.8.4 (echo variant):
//
//	detect_time = DetectMult * EchoInterval()
//
// Returns zero when echo is not active so the engine knows to skip
// the detection check entirely.
func (m *Machine) EchoDetectInterval() time.Duration {
	if !m.EchoEnabled() {
		return 0
	}
	interval := m.EchoInterval()
	if interval <= 0 {
		return 0
	}
	return time.Duration(m.vars.DetectMult) * interval
}

// EchoDetectionExpired reports whether any outstanding echo has
// been waiting longer than EchoDetectInterval. The engine calls
// this from echoTickLocked after every TX pass; a true return
// drives the session to Down with DiagEchoFailed.
//
// The check walks the full ring because slots are not ordered by
// sentAt (RegisterEchoTx overwrites the oldest slot when full, so
// the insertion order is broken once wrapping begins). With a cap
// of 16 slots the walk is trivially bounded.
func (m *Machine) EchoDetectionExpired(now time.Time) bool {
	detect := m.EchoDetectInterval()
	if detect <= 0 {
		return false
	}
	cutoff := now.Add(-detect)
	for i := range m.echoOutstanding {
		e := &m.echoOutstanding[i]
		if e.sentAt.IsZero() {
			continue
		}
		if e.sentAt.Before(cutoff) {
			return true
		}
	}
	return false
}

// EchoFail transitions the session to Down with DiagEchoFailed
// (RFC 5880 Section 4.1 diagnostic 2). Used by the engine when
// EchoDetectionExpired reports stale outstanding echoes. The
// state transition fires the notify callback exactly once so
// subscribers see the echo-originated teardown with the correct
// diagnostic instead of inheriting DiagControlDetectExpired from
// the parallel Control-path detection timer.
//
// Idempotent: a session already in Down or AdminDown is left
// alone. Clears the outstanding ring so the next Up transition
// starts with an empty detection window.
func (m *Machine) EchoFail() {
	if m.vars.SessionState != packet.StateInit && m.vars.SessionState != packet.StateUp {
		return
	}
	prev := m.vars.SessionState
	m.vars.LocalDiag = packet.DiagEchoFailed
	m.vars.SessionState = packet.StateDown
	m.nextEchoAt = time.Time{}
	for i := range m.echoOutstanding {
		m.echoOutstanding[i] = echoEntry{}
	}
	m.onStateChange(prev)
}
