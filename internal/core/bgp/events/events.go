// Design: docs/architecture/api/process-protocol.md -- BGP event types

// Package events defines event constants for the BGP component.
// This is a leaf package with no dependencies, safe to import from anywhere.
package events

// Namespace is the event namespace for the BGP component.
const Namespace = "bgp"

// BGP event types.
const (
	EventUpdate             = "update"
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
)

// Built-in send types. A send type names a message kind a process may generate
// toward a peer, which is a permission rather than a subscription. Plugins add
// more through events.RegisterSendType (e.g. "enhanced-refresh").
//
// SendRaw names a whole BGP message the caller chose, an OPEN or a NOTIFICATION
// included, written to the peer's socket with nothing built around it. It is the
// widest permission in the list, so it is granted on its own line and never
// implied by another type.
const (
	SendUpdate  = "update"
	SendRefresh = "refresh"
	SendRaw     = "raw"
)

// BaseSendTypes returns the built-in send types. The config parser and the
// YANG validator both read this list, so a type added here reaches both.
func BaseSendTypes() []string {
	return []string{SendUpdate, SendRefresh, SendRaw}
}
