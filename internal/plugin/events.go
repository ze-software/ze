// Design: docs/architecture/api/process-protocol.md — plugin process management

package plugin

// Event namespaces.
const (
	NamespaceBGP = "bgp"
	NamespaceRIB = "rib"
)

// BGP event types.
const (
	EventUpdate       = "update"
	EventOpen         = "open"
	EventNotification = "notification"
	EventKeepalive    = "keepalive"
	EventRefresh      = "refresh"
	EventState        = "state"
	EventNegotiated   = "negotiated"
)

// RIB event types.
const (
	EventCache = "cache"
	EventRoute = "route"
)

// Direction constants for event filtering.
const (
	DirectionReceived = "received"
	DirectionSent     = "sent"
	DirectionBoth     = "both"
)

// validBgpEvents is the set of valid BGP event types.
var validBgpEvents = map[string]bool{
	EventUpdate:       true,
	EventOpen:         true,
	EventNotification: true,
	EventKeepalive:    true,
	EventRefresh:      true,
	EventState:        true,
	EventNegotiated:   true,
}

// validRibEvents is the set of valid RIB event types.
var validRibEvents = map[string]bool{
	EventCache: true,
	EventRoute: true,
}
