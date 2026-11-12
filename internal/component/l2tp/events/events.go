// Design: docs/guide/l2tp.md -- L2TP route-change event handle
// Related: ../redistribute.go -- source registration (config layer)

// Package events defines the typed EventBus handles for L2TP events: the
// subscriber route-change events and the tunnel and session lifecycle events.
// Producers (the route observer, the reactor) and consumers
// (bgp-redistribute, the session readers) each call events.Register with the
// same (namespace, eventType, T) tuple; the events registry is idempotent.
//
// Reader contract for SessionIPAssigned, SessionUp and SessionDown: the
// reactor publishes the notifications of one tunnel's sessions one at a time,
// in the order the sessions' state changed, and an in-process reader runs on
// the publishing goroutine. A reader MUST NOT cause a publication for the
// same tunnel from inside its handler, by publishing one itself or by calling
// back into the reactor to end one of the tunnel's sessions: that publication would
// wait behind the one its own caller is still delivering, and the tunnel's
// notifications would stop. A reader that is slow delays the later
// notifications of its own tunnel and the goroutine that carries them.
package events

import (
	"net/netip"
	"time"

	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/redistevents"
)

// Namespace is the event namespace for L2TP route-change events.
const Namespace = "l2tp"

// ProtocolID is the numeric identity allocated for L2TP by the
// redistevents registry. Used by the observer to fill
// RouteChangeBatch.Protocol.
var ProtocolID = redistevents.RegisterProtocol(Namespace)

// registerProducer marks L2TP as having a producer so
// bgp-redistribute discovers it via redistevents.Producers().
var _ = registerProducer()

func registerProducer() bool {
	redistevents.RegisterProducer(ProtocolID)
	return true
}

// RouteChange is the typed handle for (l2tp, route-change). The
// observer emits via this handle; bgp-redistribute subscribes via its
// own local handle bound to the same (namespace, eventType, T) tuple.
var RouteChange = events.Register[*redistevents.RouteChangeBatch](Namespace, redistevents.EventType)

// SessionDownEvent is the event type string for session-down notifications.
const SessionDownEvent = "session-down"

// SessionDownPayload carries the tunnel/session IDs of a torn-down session,
// or of a session whose network lifetime ended while LCP renegotiates.
// RADIUS accounting sends its Stop; the shaper and the CoS handler drop their
// per-session traffic state; the subscriber bridge removes the subscriber entry and
// re-publishes on the subscriber namespace, which the address pool reads to
// release the address; the CQM observer uses Username to transition the
// login's bucket state to "down".
type SessionDownPayload struct {
	TunnelID  uint16
	SessionID uint16
	Username  string
	// Cause is why the session ended, as an RFC 2866 Section 5.10 value. The
	// RADIUS accounting plugin reports it as Acct-Terminate-Cause on the Stop
	// record. TerminateCauseUnspecified means no teardown site named one.
	Cause TerminateCause
}

// SessionDown is the typed handle for (l2tp, session-down). Emitted by
// the reactor when a session is removed from its tunnel, and when LCP goes
// down in the network phase; its readers are the ones SessionDownPayload
// names. The pool does not read it.
var SessionDown = events.Register[*SessionDownPayload](Namespace, SessionDownEvent)

// SessionUpEvent is the event type string for session-up notifications.
const SessionUpEvent = "session-up"

// SessionUpPayload carries session identity and the pppN interface name.
// Emitted by the reactor when PPP LCP, authentication, and all enabled
// NCPs complete successfully (ppp.EventSessionUp).
type SessionUpPayload struct {
	TunnelID        uint16
	SessionID       uint16
	Interface       string
	AccessInterface string // access VLAN name (e.g. "eth0.100"); empty for pure LNS
}

// SessionUp is the typed handle for (l2tp, session-up). Consumed by
// the shaper plugin to apply TC rules, the CoS handler, the subscriber
// bridge and the CQM observer.
var SessionUp = events.Register[*SessionUpPayload](Namespace, SessionUpEvent)

// SessionRateChangeEvent is the event type string for rate-change notifications.
const SessionRateChangeEvent = "session-rate-change"

// SessionRateChangePayload carries updated bandwidth for a session.
// Emitted by the CoA handler in the RADIUS plugin when a RADIUS server
// sends a CoA-Request with bandwidth attributes.
type SessionRateChangePayload struct {
	TunnelID     uint16
	SessionID    uint16
	DownloadRate uint64 // bits per second
	UploadRate   uint64 // bits per second
}

// SessionRateChange is the typed handle for (l2tp, session-rate-change).
// Consumed by the shaper plugin to update TC rules on the session's pppN.
var SessionRateChange = events.Register[*SessionRateChangePayload](Namespace, SessionRateChangeEvent)

// SessionCoSChangeEvent is the event type string for CoS profile change notifications.
const SessionCoSChangeEvent = "session-cos-change"

// SessionCoSChangePayload carries the new CoS profile name for a session.
// Emitted by the CoA handler when a RADIUS server sends a CoA-Request
// with a Filter-Id carrying a "cos:" prefix.
type SessionCoSChangePayload struct {
	TunnelID        uint16
	SessionID       uint16
	AccessInterface string
	ProfileName     string
}

// SessionCoSChange is the typed handle for (l2tp, session-cos-change).
var SessionCoSChange = events.Register[*SessionCoSChangePayload](Namespace, SessionCoSChangeEvent)

// EchoRTTEvent is the event type string for echo round-trip time notifications.
const EchoRTTEvent = "echo-rtt"

// EchoRTTPayload carries one LCP echo round-trip measurement.
// Emitted by the reactor on each Echo-Reply; consumed by the CQM
// aggregator to build 100s min/avg/max/loss buckets.
type EchoRTTPayload struct {
	TunnelID  uint16
	SessionID uint16
	RTT       time.Duration
	Username  string
}

// EchoRTT is the typed handle for (l2tp, echo-rtt).
var EchoRTT = events.Register[*EchoRTTPayload](Namespace, EchoRTTEvent)

// TunnelUpEvent is the event type string for tunnel-up notifications.
const TunnelUpEvent = "tunnel-up"

// TunnelUpPayload carries tunnel identity when a tunnel reaches established.
type TunnelUpPayload struct {
	TunnelID     uint16
	PeerAddr     string
	PeerHostName string
}

// TunnelUp is the typed handle for (l2tp, tunnel-up).
var TunnelUp = events.Register[*TunnelUpPayload](Namespace, TunnelUpEvent)

// TunnelDownEvent is the event type string for tunnel-down notifications.
const TunnelDownEvent = "tunnel-down"

// TunnelDownPayload carries tunnel identity and reason on teardown.
type TunnelDownPayload struct {
	TunnelID uint16
	Reason   string
}

// TunnelDown is the typed handle for (l2tp, tunnel-down).
var TunnelDown = events.Register[*TunnelDownPayload](Namespace, TunnelDownEvent)

// SessionIPAssignedEvent is the event type string for session-ip-assigned notifications.
const SessionIPAssignedEvent = "session-ip-assigned"

// SessionIPAssignedPayload carries session identity and assigned IP
// for RADIUS accounting start and other subscribers.
type SessionIPAssignedPayload struct {
	TunnelID  uint16
	SessionID uint16
	Username  string
	PeerAddr  string
	// CallingStationID is the L2TP Calling Number AVP (RFC 2661 Section
	// 4.4.3), which RADIUS accounting reports as Calling-Station-Id (RFC 2865
	// Section 5.31). Empty when neither side named one.
	CallingStationID string
	PppInterface     string
	InterfaceID      [8]byte
	DNSPrimary       netip.Addr
	DNSSecondary     netip.Addr
}

// SessionIPAssigned is the typed handle for (l2tp, session-ip-assigned).
// Emitted by the reactor after NCP negotiation assigns an IP to the peer.
var SessionIPAssigned = events.Register[*SessionIPAssignedPayload](Namespace, SessionIPAssignedEvent)
