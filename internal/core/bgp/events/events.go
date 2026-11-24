// Design: docs/architecture/api/process-protocol.md -- BGP event types

// Package events defines event constants for the BGP component.
// This is a leaf package with no dependencies, safe to import from anywhere.
package events

// Namespace is the event namespace for the BGP component.
const Namespace = "bgp"

// BGP event types.
const (
	EventUpdate             = "update"
	EventUpdateFiltered     = "update-filtered"       // RFC 7947 Section 2.1: route-server client UPDATE denied on ingress, retained in Adj-RIB-In
	EventUpdatePrePolicyOut = "update-pre-policy-out" // RFC 8671 Section 5.2: pre-policy Adj-RIB-Out candidate UPDATE before outbound policy
	EventOpen               = "open"
	EventNotification       = "notification"
	EventKeepalive          = "keepalive"
	EventRefresh            = "refresh"
	EventState              = "state"
	EventNegotiated         = "negotiated"
	EventEOR                = "eor"
	EventCongested          = "congested"
	EventResumed            = "resumed"
	EventRPKI               = "rpki"
	EventListenerReady      = "listener-ready"      // BGP reactor: TCP listener bound and accepting
	EventUpdateNotification = "update-notification" // Lightweight observability notification for UPDATE arrivals
	EventConnected          = "connected"           // BGP reactor: a peer's TCP connection is established, before any OPEN
	EventFSM                = "fsm"                 // BGP reactor: a peer's FSM moved from one RFC 4271 state to another
	EventSignal             = "signal"              // BGP reactor: the daemon took an operating-system signal, told per peer
)

// Built-in BGP send message kinds. Under the unified bus message registry
// (A-5), update and refresh are single bidirectional messages in the "bgp"
// namespace carrying tag direction [ received | sent ], while raw and
// enhanced-refresh are registered directly in "bgp".
//
// SendRaw names a whole BGP message the caller chose, an OPEN or a NOTIFICATION
// included, written to the peer's socket with nothing built around it. It is the
// widest permission in the list, so it is granted on its own line and never
// implied by another type.
const (
	SendUpdate          = "update"
	SendRefresh         = "refresh"
	SendRaw             = "raw"
	SendEnhancedRefresh = "enhanced-refresh"
)

// BaseSendTypes returns the built-in send types. The config parser and the
// YANG validator both read this list, so a type added here reaches both.
func BaseSendTypes() []string {
	return []string{SendUpdate, SendRefresh, SendRaw}
}
