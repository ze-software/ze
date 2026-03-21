// Design: docs/architecture/api/process-protocol.md — plugin process management

package plugin

import (
	"sort"
	"strings"
)

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
	EventEOR          = "eor"
	EventCongested    = "congested"
	EventResumed      = "resumed"
	EventRPKI         = "rpki"
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

// ValidBgpEvents is the set of valid BGP event types.
var ValidBgpEvents = map[string]bool{
	EventUpdate:       true,
	EventOpen:         true,
	EventNotification: true,
	EventKeepalive:    true,
	EventRefresh:      true,
	EventState:        true,
	EventNegotiated:   true,
	EventEOR:          true,
	EventCongested:    true,
	EventResumed:      true,
	EventRPKI:         true,
}

// ValidRibEvents is the set of valid RIB event types.
var ValidRibEvents = map[string]bool{
	EventCache: true,
	EventRoute: true,
}

// ValidEvents maps namespace to its set of valid event types.
// This is the single source of truth for namespace/event validation.
var ValidEvents = map[string]map[string]bool{
	NamespaceBGP: ValidBgpEvents,
	NamespaceRIB: ValidRibEvents,
}

// ValidEventNames returns a sorted, comma-separated list of valid event types
// for the given namespace. Used in error messages so the list stays in sync
// with the registration maps.
func ValidEventNames(namespace string) string {
	m := ValidEvents[namespace]
	if len(m) == 0 {
		return ""
	}
	names := make([]string, 0, len(m))
	for k := range m {
		names = append(names, k)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

// ValidNamespaceNames returns a sorted, comma-separated list of valid namespaces.
func ValidNamespaceNames() string {
	names := make([]string, 0, len(ValidEvents))
	for k := range ValidEvents {
		names = append(names, k)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}
