// Design: rfc/short/rfc5880.md -- BFD session state variables (Section 6.8.1)
// Detail: fsm.go -- reception procedure and transitions
// Detail: timers.go -- detection-time and transmit-deadline arithmetic
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
	"codeberg.org/thomas-mangin/ze/internal/plugins/bfd/auth"
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

	// XmitAuthSeq is RFC 5880 §6.8.1 bfd.XmitAuthSeq: the sequence
	// number the local end includes in the next authenticated
	// Control packet. Advanced by engine.sendLocked after each
	// periodic TX via Machine.AdvanceAuthSeq, and persisted through
	// auth.SeqPersister so restart-to-replay is survivable.
	XmitAuthSeq uint32

	// DesiredMinEchoTxInterval is RFC 5880 Section 6.8.1
	// bfd.DesiredMinEchoTxInterval: the rate at which the local end
	// wants to transmit echo packets. Zero disables echo.
	DesiredMinEchoTxInterval uint32

	// RequiredMinEchoRxInterval is RFC 5880 Section 6.8.1
	// bfd.RequiredMinEchoRxInterval: the minimum rate at which the
	// local end is willing to receive echo packets. Zero tells the
	// peer "do not send me echo packets." When non-zero, it appears
	// in outbound Control packets so peers learn the local echo
	// capability.
	RequiredMinEchoRxInterval uint32

	// RemoteMinEchoRxInterval captures the peer's advertised
	// RequiredMinEchoRxInterval. Non-zero means the peer will
	// echo back our packets; combined with a non-zero local
	// DesiredMinEchoTxInterval this enables echo TX on the tick.
	RemoteMinEchoRxInterval uint32

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

	// authPair holds the signer / verifier / persister bundle for
	// authenticated sessions (RFC 5880 §6.7). Installed via
	// SetAuth before the first send or receive; nil for
	// unauthenticated sessions.
	authPair *AuthPair

	// rcvAuthSeq tracks bfd.RcvAuthSeq for the receive-side replay
	// protection. Advanced by Verify on successful authentication.
	rcvAuthSeq auth.SeqState

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

	// nextEchoAt is the deadline for the next RFC 5880 §6.4 echo
	// packet. Zero means echo is not scheduled (session down or echo
	// not negotiated). Advanced by AdvanceEcho after every echo TX.
	nextEchoAt time.Time

	// echoSequence is the monotonic counter carried inside the ZEEC
	// envelope's Sequence field. Incremented by NextEchoSequence on
	// every echo TX so the receive path (a reflected echo hitting
	// the RTT histogram) has a unique identifier per packet.
	echoSequence uint32

	// lastEchoRTT is the most recently observed echo round-trip time.
	// Zero until the first reflected echo is matched. Exposed via
	// LastEchoRTT for snapshot consumers.
	lastEchoRTT time.Duration

	// echoSlowdownApplied tracks whether the RFC 5880 §6.8.9 echo
	// slow-down is in effect. When true, DesiredMinTxInterval and
	// RequiredMinRxInterval are raised to max(1s, configured) so the
	// Control-path rate drops while echo handles sub-second detection.
	// The flag prevents re-applying on every tick and ensures
	// RevertEchoSlowdown only fires the restore+Poll once.
	echoSlowdownApplied bool

	// echoOutstanding is a fixed-size ring of echo TX entries that
	// have not yet been matched by a returning reflection. The ring
	// is the state that turns echo from a passive RTT probe into an
	// active liveness channel: the engine declares the session Down
	// with DiagEchoFailed when the oldest unreturned entry has been
	// waiting longer than DetectMult * EchoInterval. Stage 6b Phase B.
	//
	// Slot semantics: sentAt.IsZero() marks an empty slot. The ring
	// is sized at echoOutstandingCap which is generous for the
	// default DetectMult=3 (the detection threshold is six outstanding
	// entries in the worst case). A full ring overwrites the oldest
	// slot and the overwritten entry is treated as a miss.
	echoOutstanding [echoOutstandingCap]echoEntry
}

// echoOutstandingCap is the fixed ring size for per-session echo
// TX tracking. Chosen to comfortably exceed 2 * DetectMult for the
// common DetectMult=3 without making the Machine struct fat; the
// current cap of 16 uses 16 * 24 = 384 bytes per session on a
// 64-bit build.
const echoOutstandingCap = 16

// echoEntry is one outstanding echo TX slot. sentAt is the monotonic
// time the engine handed the packet to the transport; sequence is
// the ZEEC Sequence field for the matching RX lookup. A zero sentAt
// marks the slot as empty (swept by MatchEchoRx or ClearEchoSchedule).
type echoEntry struct {
	sequence uint32
	sentAt   time.Time
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
		RequiredMinEchoRxInterval:       req.DesiredMinEchoTxInterval,
		DesiredMinEchoTxInterval:        req.DesiredMinEchoTxInterval,
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

// MinTTL returns the minimum acceptable receive TTL for the session.
//
// For multi-hop sessions this is the RFC 5883 Section 5 weak-GTSM floor;
// zero in the configuration request defaults to 254 (one hop allowed
// beyond the peer's first hop), matching the ze-bfd-conf.yang default.
// For single-hop sessions RFC 5881 Section 5 mandates TTL == 255 and this
// value is not consulted by the engine TTL gate.
func (m *Machine) MinTTL() uint8 {
	if m.configReq.MinTTL == 0 {
		return 254
	}
	return m.configReq.MinTTL
}

// DetectMult returns the local bfd.DetectMult used in outgoing packets.
// Exposed for the engine's RFC 5880 Section 6.8.7 jitter calculation:
// when DetectMult == 1 the reduction has a 10% floor so the receiver
// never detects before the next packet arrives.
func (m *Machine) DetectMult() uint8 { return m.vars.DetectMult }

// LocalDiag returns the current bfd.LocalDiag (RFC 5880 Section 4.1).
// Exposed so observability consumers (Snapshot, show bfd sessions)
// can report the latest diagnostic without touching the unexported
// Vars struct.
func (m *Machine) LocalDiag() packet.Diag { return m.vars.LocalDiag }

// RemoteMinRxInterval returns the peer's most recently advertised
// bfd.RequiredMinRxInterval as a Go time.Duration. Zero until the
// first Control packet is received.
func (m *Machine) RemoteMinRxInterval() time.Duration {
	return time.Duration(m.vars.RemoteMinRxInterval) * time.Microsecond
}
