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

	"github.com/ze-software/ze/internal/component/bfd/packet"
)

// DetectionInterval returns the current detection time as a Go duration.
//
// RFC 5880 Section 6.8.4 (Asynchronous mode):
//
//	detect_time = remote_detect_mult * max(local_RequiredMinRx, remote_DesiredMinTx)
func (m *Machine) DetectionInterval() time.Duration {
	mult := uint32(m.vars.RemoteDetectMult)
	if mult == 0 {
		mult = uint32(m.vars.DetectMult)
	}
	floor := max(m.vars.RequiredMinRxInterval, m.vars.RemoteDesiredMinTx)
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
// Jitter is applied per-packet by the engine, not here. The
// bfd.DesiredMinTxInterval it uses is the one in force
// (desiredMinTxInForceUs), which lags an increase until its Poll Sequence
// terminates.
func (m *Machine) TransmitInterval() time.Duration {
	tx := max(m.desiredMinTxInForceUs(), m.vars.RemoteMinRxInterval)
	if tx == 0 {
		tx = SlowStartIntervalUs
	}
	return time.Duration(tx) * time.Microsecond
}

// desiredMinTxInForceUs answers the bfd.DesiredMinTxInterval the actual
// transmission interval is computed from: the held pre-increase value while an
// increase made in Up waits for its Poll Sequence, the live variable
// otherwise.
//
// RFC 5880 Section 6.8.3: "If bfd.DesiredMinTxInterval is increased and
// bfd.SessionState is Up, the actual transmission interval used MUST NOT
// change until the Poll Sequence described above has terminated." A decrease
// takes effect at once and is never held.
func (m *Machine) desiredMinTxInForceUs() uint32 {
	if m.txDesiredHeld {
		return m.txDesiredHeldUs
	}
	return m.vars.DesiredMinTxInterval
}

// holdDesiredMinTxLocked records the bfd.DesiredMinTxInterval in force before
// a write of raisedUs, when that write is an increase made in Up. The caller
// MUST call it before the write, and MUST start the Poll Sequence that carries
// the new value in the same step. A hold already in place keeps its value: it
// is the interval in force until the first Poll terminates.
func (m *Machine) holdDesiredMinTxLocked(raisedUs uint32) {
	if m.vars.SessionState != packet.StateUp {
		return
	}
	if raisedUs <= m.vars.DesiredMinTxInterval {
		return
	}
	if m.txDesiredHeld {
		return
	}
	m.txDesiredHeld = true
	m.txDesiredHeldUs = m.vars.DesiredMinTxInterval
}

// releaseDesiredMinTxLocked ends a hold when the Poll Sequence terminates or
// the session leaves Up, and moves the pending TX deadline to the interval
// that is now in force (RFC 5880 Section 6.8.7).
func (m *Machine) releaseDesiredMinTxLocked() {
	if !m.txDesiredHeld {
		return
	}
	m.txDesiredHeld = false
	m.rescheduleTxLocked()
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
// with diagnostic 1 (Control Detection Time Expired). In Down or AdminDown
// the expiry only clears bfd.RemoteDiscr.
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
		// RFC 5880 Section 6.8.1: "If a period of a Detection Time passes
		// without the receipt of a valid, authenticated BFD packet from the
		// remote system, this variable MUST be set to zero." The rule holds
		// in every state: a peer that signaled Down and then fell silent
		// loses its discriminator here, with no state change to report.
		m.vars.RemoteDiscr = 0
		m.nextDetectAt = time.Time{}
		// RFC 5880 Section 6.8.7.
		if !m.transmitPermitted() {
			m.nextTxAt = time.Time{}
		}
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
	m.lastTxAt = now
	m.txInterval = interval
	m.txWait = interval - reduction
	m.nextTxAt = now.Add(m.txWait)
}

// rescheduleTxLocked moves the pending periodic TX deadline when the
// transmit interval differs from m.txInterval, the interval the pending
// wait was drawn from. Every writer of bfd.DesiredMinTxInterval or
// bfd.RemoteMinRxInterval that the RFC wants effected immediately MUST
// call it after the write.
//
// RFC 5880 Section 6.8.7: "The transmit interval MUST be recalculated
// whenever bfd.DesiredMinTxInterval changes, or whenever
// bfd.RemoteMinRxInterval changes, and is equal to the greater of those two
// values."
//
// RFC 5880 Section 6.8.3: "In other words, the local system cannot wait
// longer than the new interval between the previous packet transmission and
// the next one. If this interval has already passed since the last
// transmission (because the new interval is significantly shorter), the
// local system MUST send the next periodic BFD Control packet as soon as
// practicable."
//
// The jittered wait is scaled by the ratio of the two intervals, so the
// jitter drawn for the last packet keeps its share of the new interval and
// the 0 to 25 percent reduction of Section 6.8.7 still holds. A deadline in
// the past fires on the next tick. A deadline earlier than the periodic one
// is an immediate send a state change asked for, and is left alone.
func (m *Machine) rescheduleTxLocked() {
	txInterval := m.TransmitInterval()
	txIntervalPrev := m.txInterval
	if txInterval == txIntervalPrev {
		return
	}
	if m.nextTxAt.IsZero() {
		return
	}
	if m.lastTxAt.IsZero() {
		return
	}
	if m.nextTxAt.Before(m.lastTxAt.Add(m.txWait)) {
		return
	}
	// Both intervals are at most 2^32-1 microseconds, and the wait is at
	// most the previous interval, so the product fits in a uint64. The
	// division rounds down.
	waitUs := uint64(m.txWait / time.Microsecond)
	prevUs := uint64(txIntervalPrev / time.Microsecond)
	nextUs := uint64(txInterval / time.Microsecond)
	m.txWait = time.Duration(waitUs*nextUs/prevUs) * time.Microsecond
	m.txInterval = txInterval
	m.nextTxAt = m.lastTxAt.Add(m.txWait)
}

// transmitPermitted reports whether the session may send a Control packet.
//
// RFC 5880 Section 6.8.7: "A system MUST NOT transmit BFD Control packets if
// bfd.RemoteDiscr is zero and the system is taking the Passive role." An
// Active session may always transmit.
func (m *Machine) transmitPermitted() bool {
	if m.role == RoleActive {
		return true
	}
	return m.vars.RemoteDiscr != 0
}

// LastReceived returns the timestamp of the most recently accepted Control
// packet, or zero if none has been received.
func (m *Machine) LastReceived() time.Time { return m.lastRxTime }

// PollOutstanding reports whether the session is currently sending Poll
// packets and waiting for an F-bit reply.
func (m *Machine) PollOutstanding() bool { return m.vars.PollOutstanding }

// pollSequenceIdle reports whether a new Poll Sequence may start: no Poll is
// outstanding, and a Control packet with the Final bit clear has arrived
// since the last Poll Sequence completed.
//
// RFC 5880 Section 6.8.3: "3) an additional BFD Control packet with the
// Final (F) bit *clear* MUST be received after the Poll Sequence has
// completed prior to the initiation of another Poll Sequence".
func (m *Machine) pollSequenceIdle() bool {
	if m.vars.PollOutstanding {
		return false
	}
	return !m.pollSettling
}

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
	m.lastEchoAt = now
	m.nextEchoAt = now.Add(interval)
}

// rescheduleEchoLocked moves the pending echo deadline to the previous echo
// transmission plus the current EchoInterval. Receive calls it when the peer
// changes Required Min Echo RX Interval, so a raised floor delays the next
// echo rather than taking effect one echo late.
//
// RFC 5880 Section 6.8.9: "The interval between transmitted BFD Echo packets
// MUST NOT be less than the value advertised by the remote system in Required
// Min Echo RX Interval".
//
// No echo scheduled, or none sent yet, leaves the deadline alone: PrimeEcho
// owns the first echo. A peer that stops advertising echo disables it, and
// the deadline is cleared so no echo leaves before the engine's next tick.
func (m *Machine) rescheduleEchoLocked() {
	if m.nextEchoAt.IsZero() {
		return
	}
	if m.lastEchoAt.IsZero() {
		return
	}
	interval := m.EchoInterval()
	if interval <= 0 {
		m.nextEchoAt = time.Time{}
		return
	}
	m.nextEchoAt = m.lastEchoAt.Add(interval)
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
	m.lastEchoAt = time.Time{}
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
//
// While a Poll Sequence is outstanding or settling (pollSequenceIdle)
// the slow-down is deferred, not dropped: the flag stays clear and the
// engine's next echo tick calls again. RFC 5880 Section 6.8.3: "if
// multiple changes are made that require the use of a Poll Sequence,
// there are three choices". The Up Poll that onStateChange starts is
// already carrying its own change, so this second change takes choice 3
// rather than riding a Poll whose Final would answer both.
func (m *Machine) ApplyEchoSlowdown() {
	if m.echoSlowdownApplied {
		return
	}
	// RFC 5880 Section 6.8.3
	if !m.pollSequenceIdle() {
		return
	}
	m.echoSlowdownApplied = true
	slowedUs := max(EchoSlowdownIntervalUs, m.vars.ConfiguredDesiredMinTxInterval)
	// RFC 5880 Section 6.8.3
	m.holdDesiredMinTxLocked(slowedUs)
	m.vars.DesiredMinTxInterval = slowedUs
	m.vars.RequiredMinRxInterval = max(EchoSlowdownIntervalUs, m.vars.ConfiguredRequiredMinRxInterval)
	m.vars.PollOutstanding = true
}

// revertEchoSlowdownLocked restores the configured intervals, moves the
// pending TX deadline to the restored transmit interval, and initiates a
// Poll if the slow-down was active. Safe to call when the slow-down is not
// applied (no-op).
//
// While Up with a Poll Sequence outstanding or settling (pollSequenceIdle)
// the revert is deferred with the flag still set, and ClearEchoSchedule,
// which the engine runs on every tick while echo is off, calls again. The
// Poll that applied the slow-down is usually the one still outstanding.
func (m *Machine) revertEchoSlowdownLocked() {
	if !m.echoSlowdownApplied {
		return
	}
	if m.vars.SessionState == packet.StateUp {
		// RFC 5880 Section 6.8.3
		if !m.pollSequenceIdle() {
			return
		}
	}
	m.echoSlowdownApplied = false
	m.vars.DesiredMinTxInterval = m.vars.ConfiguredDesiredMinTxInterval
	m.vars.RequiredMinRxInterval = m.vars.ConfiguredRequiredMinRxInterval
	// RFC 5880 Section 6.8.7: the transmit interval is recalculated when
	// bfd.DesiredMinTxInterval changes, and Section 6.8.3 makes a reduction
	// take effect immediately, so the pending deadline moves now.
	m.rescheduleTxLocked()
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
