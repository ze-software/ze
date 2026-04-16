// Design: docs/research/l2tpv2-ze-integration.md -- PPP -> transport event boundary

package ppp

// Event is the sealed sum type emitted on Manager.EventsOut. The
// transport (l2tp today) reads this channel in its select loop and
// reacts: EventLCPDown / EventSessionDown trigger a CDN; EventSessionUp
// is informational; future EventAuthRequest / EventIPRequest (specs 6b,
// 6c) feed plugins.
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
// error). Reason is human-readable for logs; the transport should
// treat any LCP-Down as session teardown.
type EventLCPDown struct {
	TunnelID  uint16
	SessionID uint16
	Reason    string
}

func (EventLCPDown) isPPPEvent() {}

// EventSessionUp is emitted when LCP and authentication and at least
// one NCP have completed successfully and pppN is configured.
//
// In Phase 6a the auth-phase hook is stubbed to always succeed and
// NCPs are not yet wired, so this event fires immediately after the
// stub auth returns. Specs 6b and 6c gate it on real auth and NCP
// completion respectively.
type EventSessionUp struct {
	TunnelID  uint16
	SessionID uint16
}

func (EventSessionUp) isPPPEvent() {}

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
