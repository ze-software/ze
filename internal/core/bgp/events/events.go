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
