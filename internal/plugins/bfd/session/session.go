// Design: rfc/short/rfc5880.md -- BFD session state variables (Section 6.8.1)
//
// Package session implements the per-BFD-session state machine, timer
// arithmetic, and Poll/Final negotiation. It contains no I/O: callers
// drive incoming packets through Receive and consume outgoing packets
// through Build. Wiring to UDP sockets lives in the transport package.
//
// A Machine is NOT safe for concurrent use. The express-loop pattern
// (BIRD style, see internal/plugins/bfd/engine) gives every session a
// single owning goroutine.
package session

import (
	"net/netip"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/core/clock"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bfd/api"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bfd/packet"
)

// SlowStartIntervalUs is the minimum bfd.DesiredMinTxInterval while a
// session is not Up. RFC 5880 Section 6.8.3 mandates >= 1 000 000 us.
const SlowStartIntervalUs uint32 = 1_000_000

// DefaultDetectMult is the default detection multiplier when a client
// passes zero as its DetectMult (see api.SessionRequest).
const DefaultDetectMult uint8 = 3

// Vars is the canonical set of bfd.* state variables from RFC 5880
// Section 6.8.1. Field names mirror the RFC for cross-reference.
//
// All time intervals are in microseconds.
type Vars struct {
	SessionState       packet.State
	RemoteSessionState packet.State

	LocalDiscr  uint32
	RemoteDiscr uint32

	LocalDiag packet.Diag

	DesiredMinTxInterval  uint32 // local target transmit rate
	RequiredMinRxInterval uint32 // local receive capacity
	RemoteMinRxInterval   uint32 // last advertised by peer

	// ConfiguredDesiredMinTxInterval and ConfiguredRequiredMinRxInterval
	// hold the operating values the client asked for. While not Up, the
	// session uses SlowStartIntervalUs and only switches to these via a
	// Poll/Final exchange after reaching Up.
	ConfiguredDesiredMinTxInterval  uint32
	ConfiguredRequiredMinRxInterval uint32

	DemandMode       bool
	RemoteDemandMode bool

	DetectMult       uint8
	RemoteDetectMult uint8

	AuthType uint8

	// PollOutstanding is true while the local end is sending packets
	// with the P bit set, awaiting an F-bit reply.
	PollOutstanding bool
}

// Machine is a per-peer BFD session: state variables, identity, refcount,
// and the most recent inbound packet timestamp used by detection-time
// arithmetic.
//
// Caller MUST call Init exactly once before any other method.
type Machine struct {
	// Identity (immutable after Init)
	key       api.Key
	role      Role
	clk       clock.Clock
	notify    func(packet.State, packet.Diag)
	configReq api.SessionRequest

	// State (mutable, owned by the express-loop goroutine)
	vars       Vars
	lastRxTime time.Time
	createdAt  time.Time
	refcount   int

	// nextDetectAt is the deadline for the detection timer. Zero means
	// detection is not armed (slow start has not yet received a packet,
	// or session is AdminDown).
	nextDetectAt time.Time

	// nextTxAt is the deadline for the next periodic Control packet.
	nextTxAt time.Time
}

// Role is the BFD role (Active or Passive) per RFC 5883 Section 4.3.
type Role uint8

const (
	// RoleActive systems transmit Control packets immediately on session
	// creation. The default for both single-hop endpoints and the sender
	// in a unidirectional multi-hop pair.
	RoleActive Role = iota
	// RolePassive systems transmit nothing until they receive a Control
	// packet from the peer. Used by the receiver in a unidirectional
	// multi-hop pair (RFC 5883 Section 4.3).
	RolePassive
)

// Init prepares the session for use.
//
// localDiscr MUST be a unique nonzero discriminator. notify is called from
// the session goroutine on every state transition with the new state and
// the diagnostic recorded in bfd.LocalDiag at the moment of the change.
// clk supplies the time source; production code passes clock.RealClock{},
// tests inject a fake clock to drive timer transitions deterministically.
//
// Init sets bfd.SessionState to Down, applies slow-start intervals, and
// arms the next-TX timer for the active role. Passive sessions stay
// silent until the first received packet.
func (m *Machine) Init(req api.SessionRequest, localDiscr uint32, clk clock.Clock, notify func(packet.State, packet.Diag)) {
	if clk == nil {
		clk = clock.RealClock{}
	}
	if notify == nil {
		notify = func(packet.State, packet.Diag) {}
	}

	m.key = req.Key()
	m.configReq = req
	m.role = RoleActive
	if req.Passive {
		m.role = RolePassive
	}
	m.clk = clk
	m.notify = notify
	m.refcount = 1

	mult := req.DetectMult
	if mult == 0 {
		mult = DefaultDetectMult
	}
	configTx := req.DesiredMinTxInterval
	if configTx == 0 {
		configTx = SlowStartIntervalUs
	}
	configRx := req.RequiredMinRxInterval
	if configRx == 0 {
		configRx = SlowStartIntervalUs
	}

	m.vars = Vars{
		SessionState:                    packet.StateDown,
		RemoteSessionState:              packet.StateDown,
		LocalDiscr:                      localDiscr,
		RemoteDiscr:                     0,
		LocalDiag:                       packet.DiagNone,
		DesiredMinTxInterval:            SlowStartIntervalUs,
		RequiredMinRxInterval:           configRx,
		RemoteMinRxInterval:             1, // RFC 5880 Section 6.8.1 init
		ConfiguredDesiredMinTxInterval:  configTx,
		ConfiguredRequiredMinRxInterval: configRx,
		DetectMult:                      mult,
	}

	m.createdAt = m.clk.Now()
	if m.role == RoleActive {
		m.nextTxAt = m.createdAt
	}
}

// Key returns the session's identity. Safe to call after Init.
func (m *Machine) Key() api.Key { return m.key }

// State returns the current bfd.SessionState. Safe to call after Init.
func (m *Machine) State() packet.State { return m.vars.SessionState }

// LocalDiscriminator returns the local discriminator chosen at Init.
func (m *Machine) LocalDiscriminator() uint32 { return m.vars.LocalDiscr }

// RemoteDiscriminator returns the most recently learned peer discriminator,
// or zero if the peer has not yet sent a packet.
func (m *Machine) RemoteDiscriminator() uint32 { return m.vars.RemoteDiscr }

// Refcount returns the current shared-session refcount. The session is
// torn down when this drops to zero.
func (m *Machine) Refcount() int { return m.refcount }

// Acquire increments the refcount and returns the new value. Used when a
// second client asks for the same session.
func (m *Machine) Acquire() int {
	m.refcount++
	return m.refcount
}

// Release decrements the refcount and returns the new value. The caller
// is responsible for tearing the session down when this returns zero.
func (m *Machine) Release() int {
	if m.refcount > 0 {
		m.refcount--
	}
	return m.refcount
}

// PeerAddr returns the configured peer address (used by the transport
// layer to construct the destination of outgoing packets).
func (m *Machine) PeerAddr() netip.Addr { return m.configReq.Peer }
