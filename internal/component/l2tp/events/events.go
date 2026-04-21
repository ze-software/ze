// Design: docs/architecture/l2tp.md -- L2TP route-change event handle
// Related: ../redistribute.go -- source registration (config layer)

// Package events defines the typed EventBus handle for L2TP subscriber
// route-change events. Producers (the route observer) and consumers
// (bgp-redistribute) each call events.Register with the same
// (namespace, eventType, T) tuple; the events registry is idempotent.
package events

import (
	"codeberg.org/thomas-mangin/ze/internal/core/events"
	"codeberg.org/thomas-mangin/ze/internal/core/redistevents"
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

// SessionDownPayload carries the tunnel/session IDs of a torn-down session.
// Pool plugins subscribe to release allocated addresses.
type SessionDownPayload struct {
	TunnelID  uint16
	SessionID uint16
}

// SessionDown is the typed handle for (l2tp, session-down). Emitted by
// the reactor when a PPP session tears down; consumed by the pool
// plugin to release allocated IP addresses.
var SessionDown = events.Register[*SessionDownPayload](Namespace, SessionDownEvent)

// SessionUpEvent is the event type string for session-up notifications.
const SessionUpEvent = "session-up"

// SessionUpPayload carries session identity and the pppN interface name.
// Emitted by the reactor when PPP LCP, authentication, and all enabled
// NCPs complete successfully (ppp.EventSessionUp).
type SessionUpPayload struct {
	TunnelID  uint16
	SessionID uint16
	Interface string
}

// SessionUp is the typed handle for (l2tp, session-up). Consumed by
// the shaper plugin to apply TC rules and by stats plugins.
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

// SessionIPAssignedEvent is the event type string for session-ip-assigned notifications.
const SessionIPAssignedEvent = "session-ip-assigned"

// SessionIPAssignedPayload carries session identity and assigned IP
// for RADIUS accounting start and other subscribers.
type SessionIPAssignedPayload struct {
	TunnelID  uint16
	SessionID uint16
	Username  string
	PeerAddr  string
}

// SessionIPAssigned is the typed handle for (l2tp, session-ip-assigned).
// Emitted by the reactor after NCP negotiation assigns an IP to the peer.
var SessionIPAssigned = events.Register[*SessionIPAssignedPayload](Namespace, SessionIPAssignedEvent)
