// Design: rfc/short/rfc5880.md -- Reception procedure (Section 6.8.6)
// Related: session.go -- Machine state and identity
//
// Reception, transition, and outgoing-packet construction logic.
//
// The functions in this file mirror RFC 5880 Section 6.8.6 (reception)
// and Section 6.8.7 (transmission). They contain the only mutations to
// the bfd.* state variables outside of Init.
package session

import (
	"errors"

	"codeberg.org/thomas-mangin/ze/internal/plugins/bfd/packet"
)

// Errors returned by Receive.
var (
	// ErrYourDiscriminatorReset signals an RFC 5880 Section 6.8.6 violation:
	// a packet with Your Discriminator == 0 arrived for a session that is
	// not in Down or AdminDown. The engine MUST discard the packet without
	// updating any state.
	ErrYourDiscriminatorReset = errors.New("bfd: your-discriminator zero on non-Down session")

	// ErrAuthMismatch signals an authenticated/unauthenticated mismatch
	// between local configuration and the received packet. The engine MUST
	// discard the packet.
	ErrAuthMismatch = errors.New("bfd: authentication mismatch")
)

// Receive applies an RFC 5880 Section 6.8.6 reception step to the session.
//
// It updates bfd.RemoteState, bfd.RemoteDiscr, bfd.RemoteMinRxInterval, the
// detection-time deadline, and the local FSM. The notify callback is called
// exactly once if the local SessionState changed.
//
// Receive returns an error only when the packet must be discarded *after*
// having been validated by the codec; structural errors come from
// packet.ParseControl and never reach Receive. Even on a successful return,
// the caller may need to send a Final packet (see ShouldSendFinal).
func (m *Machine) Receive(c packet.Control) error {
	// RFC 5880 Section 6.8.6 zero-discriminator rule.
	if c.YourDiscriminator == 0 &&
		m.vars.SessionState != packet.StateDown &&
		m.vars.SessionState != packet.StateAdminDown {
		return ErrYourDiscriminatorReset
	}

	// Authentication mismatch (Section 6.8.6).
	wantAuth := m.vars.AuthType != 0
	if c.Auth != wantAuth {
		return ErrAuthMismatch
	}

	// Field updates that happen regardless of FSM transition.
	m.vars.RemoteDiscr = c.MyDiscriminator
	m.vars.RemoteSessionState = c.State
	m.vars.RemoteDemandMode = c.Demand
	m.vars.RemoteMinRxInterval = c.RequiredMinRxInterval
	m.vars.RemoteDetectMult = c.DetectMult

	// Section 6.8.6: terminate Poll on F=1.
	if m.vars.PollOutstanding && c.Final {
		m.vars.PollOutstanding = false
	}

	now := m.clk.Now()
	m.lastRxTime = now
	m.armDetectionLocked(now)

	// Section 6.8.6: AdminDown discard rule.
	if m.vars.SessionState == packet.StateAdminDown {
		return nil
	}

	prevState := m.vars.SessionState

	// Section 6.8.6 transition table.
	if c.State == packet.StateAdminDown {
		if m.vars.SessionState != packet.StateDown {
			m.vars.LocalDiag = packet.DiagNeighborSignaledDown
			m.vars.SessionState = packet.StateDown
		}
	} else {
		m.applyTransitionLocked(c.State)
	}

	if m.vars.SessionState != prevState {
		m.onStateChange(prevState)
	}

	return nil
}

// applyTransitionLocked runs the non-AdminDown half of the RFC 5880
// Section 6.8.6 reception table.
func (m *Machine) applyTransitionLocked(recvState packet.State) {
	switch m.vars.SessionState {
	case packet.StateDown:
		switch recvState {
		case packet.StateDown:
			m.vars.SessionState = packet.StateInit
			m.vars.LocalDiag = packet.DiagNone
		case packet.StateInit:
			m.vars.SessionState = packet.StateUp
			m.vars.LocalDiag = packet.DiagNone
		case packet.StateUp:
			// RFC 5880 Section 6.2: ignore Up while local Down.
		case packet.StateAdminDown:
			// Handled by the AdminDown branch in Receive.
		}
	case packet.StateInit:
		if recvState == packet.StateInit || recvState == packet.StateUp {
			m.vars.SessionState = packet.StateUp
			m.vars.LocalDiag = packet.DiagNone
		}
	case packet.StateUp:
		if recvState == packet.StateDown {
			m.vars.LocalDiag = packet.DiagNeighborSignaledDown
			m.vars.SessionState = packet.StateDown
		}
	case packet.StateAdminDown:
		// Already handled by caller; included for switch exhaustiveness.
	}
}

// onStateChange runs after the FSM moves into a new state. It triggers any
// timer or Poll bookkeeping that depends on the transition and notifies
// the engine.
func (m *Machine) onStateChange(prev packet.State) {
	now := m.clk.Now()

	if m.vars.SessionState == packet.StateUp {
		// RFC 5880 Section 6.8.3 / Phase 4: a session reaching Up
		// initiates a Poll Sequence to switch from slow-start
		// intervals to the configured operating values.
		if m.vars.DesiredMinTxInterval != m.vars.ConfiguredDesiredMinTxInterval ||
			m.vars.RequiredMinRxInterval != m.vars.ConfiguredRequiredMinRxInterval {
			m.vars.DesiredMinTxInterval = m.vars.ConfiguredDesiredMinTxInterval
			m.vars.RequiredMinRxInterval = m.vars.ConfiguredRequiredMinRxInterval
			m.vars.PollOutstanding = true
		}
	}

	if m.vars.SessionState == packet.StateDown && prev != packet.StateDown {
		// Section 6.8.3: while not Up, restore slow-start interval
		// floor on the local TX rate.
		m.vars.DesiredMinTxInterval = SlowStartIntervalUs
		m.vars.PollOutstanding = false
		// RFC 5880 §6.8.1: RemoteDiscr MUST be cleared when a
		// Detection Time passes without a valid packet -- NOT on
		// every Down transition. A peer-signaled Down still leaves
		// the peer reachable, and clearing its discriminator would
		// force the handshake to re-learn it when a quick recovery
		// could otherwise reuse it.
		if m.vars.LocalDiag == packet.DiagControlDetectExpired {
			m.vars.RemoteDiscr = 0
		}
	}

	// Send the next packet immediately to communicate the new state.
	m.nextTxAt = now

	m.notify(m.vars.SessionState, m.vars.LocalDiag)
}

// AdminDown forces the session into AdminDown state with the supplied
// diagnostic. The engine calls this when an operator disables the session
// or when an external signal indicates the path is down.
func (m *Machine) AdminDown(diag packet.Diag) {
	if m.vars.SessionState == packet.StateAdminDown {
		return
	}
	prev := m.vars.SessionState
	m.vars.SessionState = packet.StateAdminDown
	m.vars.LocalDiag = diag
	m.onStateChange(prev)
}

// AdminEnable transitions the session out of AdminDown back to Down so it
// can begin the handshake again. No-op if not in AdminDown.
func (m *Machine) AdminEnable() {
	if m.vars.SessionState != packet.StateAdminDown {
		return
	}
	prev := m.vars.SessionState
	m.vars.SessionState = packet.StateDown
	m.vars.LocalDiag = packet.DiagNone
	m.vars.DesiredMinTxInterval = SlowStartIntervalUs
	m.onStateChange(prev)
}

// Build fills c with the next outgoing Control packet for this session.
//
// The caller is the express-loop goroutine; Build is called either when
// the periodic TX timer fires or in response to a received Poll. The
// returned Control is ready for packet.Control.WriteTo.
func (m *Machine) Build() packet.Control {
	return packet.Control{
		Version:                   packet.Version,
		Diag:                      m.vars.LocalDiag,
		State:                     m.vars.SessionState,
		Poll:                      m.vars.PollOutstanding,
		Final:                     false,
		CPI:                       false,
		Auth:                      m.vars.AuthType != 0,
		Demand:                    m.canSetDemand(),
		Multipoint:                false,
		DetectMult:                m.vars.DetectMult,
		Length:                    packet.MandatoryLen,
		MyDiscriminator:           m.vars.LocalDiscr,
		YourDiscriminator:         m.vars.RemoteDiscr,
		DesiredMinTxInterval:      m.vars.DesiredMinTxInterval,
		RequiredMinRxInterval:     m.vars.RequiredMinRxInterval,
		RequiredMinEchoRxInterval: 0,
	}
}

// BuildFinal returns the immediate F=1 reply to a received Poll. RFC 5880
// Section 6.8.6 says this packet is sent without respect to the TX timer
// or any other limitation.
func (m *Machine) BuildFinal() packet.Control {
	c := m.Build()
	c.Poll = false
	c.Final = true
	return c
}

// canSetDemand reports whether the D bit may be set in outgoing packets.
// RFC 5880 Section 6.8.7: D MUST NOT be set unless local DemandMode is
// active and both ends are Up.
func (m *Machine) canSetDemand() bool {
	return m.vars.DemandMode &&
		m.vars.SessionState == packet.StateUp &&
		m.vars.RemoteSessionState == packet.StateUp
}
