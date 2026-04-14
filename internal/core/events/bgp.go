// Design: docs/architecture/api/process-protocol.md -- BGP event types

package events

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

// ValidBgpEvents is the set of valid BGP event types.
// Includes all types accepted in config receive flags (base + directions).
var ValidBgpEvents = map[string]bool{
	EventUpdate:             true,
	EventOpen:               true,
	EventNotification:       true,
	EventKeepalive:          true,
	EventRefresh:            true,
	EventState:              true,
	EventNegotiated:         true,
	EventEOR:                true,
	EventCongested:          true,
	EventResumed:            true,
	EventRPKI:               true,
	EventListenerReady:      true,
	EventUpdateNotification: true,
	DirectionSent:           true, // "sent" -- config receive flag for sent UPDATE events
}
