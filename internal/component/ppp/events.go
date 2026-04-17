// Design: docs/research/l2tpv2-ze-integration.md -- PPP -> transport event boundary
// Related: auth_events.go -- AuthEvent sum for auth handler channel
// Related: ip_events.go -- IPEvent sum for IP handler channel

package ppp

import "net/netip"

// Event is the sealed sum type emitted on Manager.EventsOut. The
// transport (l2tp today) reads this channel in its select loop and
// reacts: EventLCPDown / EventSessionDown trigger a CDN; EventSessionUp
// is informational; EventSessionIPAssigned carries NCP completion.
//
// Implementations are restricted to the types in this file via the
// unexported isPPPEvent method.
type Event interface {
	isPPPEvent()
}

// EventLCPUp is emitted when the LCP FSM reaches the Opened state.
// Carries the negotiated MRU so the transport can compute MTU or log
// it. Authentication and NCPs run after this point.
type EventLCPUp struct {
	TunnelID      uint16
	SessionID     uint16
	NegotiatedMRU uint16
}

func (EventLCPUp) isPPPEvent() {}

// EventLCPDown is emitted when LCP transitions out of the Opened state
// for any reason (peer Terminate-Request, Echo timeout, fatal parse
// error). Reason is human-readable for logs.
//
// INFORMATIONAL: EventLCPDown does NOT require the transport to tear
// down the L2TP session. The per-session goroutine ALWAYS exits after
// LCP closes, and that exit emits EventSessionDown, which is the
// canonical teardown signal. Transports that react to both LCPDown
// and SessionDown would double-teardown. Use LCPDown only for metrics
// and logging.
type EventLCPDown struct {
	TunnelID  uint16
	SessionID uint16
	Reason    string
}

func (EventLCPDown) isPPPEvent() {}

// EventSessionUp is emitted when LCP, authentication, and every
// enabled NCP have completed successfully and pppN is configured.
// Spec 6c moves this event from "after auth" to "after NCPs complete"
// so the transport does not declare the session usable until an IP
// address is actually on pppN.
type EventSessionUp struct {
	TunnelID  uint16
	SessionID uint16
}

func (EventSessionUp) isPPPEvent() {}

// EventSessionIPAssigned is emitted after one NCP (IPCP or IPv6CP)
// has successfully completed negotiation and ze has either programmed
// the pppN interface (IPv4) or noted the negotiated identifier (IPv6).
// The L2TP subsystem reacts by injecting the subscriber route into the
// redistribute path (wired in spec-l2tp-7-subsystem).
//
// For family=ipv4: Local and Peer are populated with the 32-bit
// addresses, DNSPrimary / DNSSecondary are optional (may be the zero
// netip.Addr), InterfaceID is zero-valued.
//
// For family=ipv6: InterfaceID holds the 8-byte EUI-64 interface
// identifier negotiated with the peer; Local / Peer / DNSPrimary /
// DNSSecondary are zero-valued. Ze does not assign a /64 via IPv6CP
// (DHCPv6-PD and SLAAC are out of umbrella scope).
type EventSessionIPAssigned struct {
	TunnelID     uint16
	SessionID    uint16
	Family       AddressFamily
	Local        netip.Addr
	Peer         netip.Addr
	DNSPrimary   netip.Addr
	DNSSecondary netip.Addr
	InterfaceID  [8]byte
}

func (EventSessionIPAssigned) isPPPEvent() {}

// EventSessionDown is emitted when a per-session goroutine that WAS
// running exits for any reason: peer-initiated teardown, local
// error, Manager.StopSession call, auth/NCP failure. The transport
// should send a CDN and release its session state.
//
// Distinct from EventSessionRejected: a SessionDown implies the
// session was at least accepted and a goroutine was spawned.
//
// Reason is human-readable for logs. The transport MUST NOT parse
// reason for control flow -- use the event type to discriminate.
type EventSessionDown struct {
	TunnelID  uint16
	SessionID uint16
	Reason    string
}

func (EventSessionDown) isPPPEvent() {}

// EventSessionRejected is emitted when a StartSession is refused
// before any per-session goroutine starts. The refusal reasons are
// transport bugs or StartSession hygiene problems: invalid file
// descriptors, duplicate (tunnelID, sessionID), etc.
//
// The transport should treat this as "PPP never owned this session"
// -- in-flight tracking should be cleared WITHOUT running the
// session-termination cleanup path (which assumes the session was
// established and has FSM/auth state). Matches the semantics the
// L2TP kernel worker would see when a session setup never reaches
// the PPP engine.
//
// Reason is human-readable for logs.
type EventSessionRejected struct {
	TunnelID  uint16
	SessionID uint16
	Reason    string
}

func (EventSessionRejected) isPPPEvent() {}
