// Design: docs/architecture/api/process-protocol.md -- plugin process management

package plugin

import (
	"fmt"
	"sort"
	"strings"
	"sync"
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

// eventsMu protects ValidEvents, ValidBgpEvents, and ValidRibEvents from
// concurrent read/write. Writes happen during RegisterEventType (startup).
// Reads happen during subscribe-events and emit-event validation (runtime).
var eventsMu sync.RWMutex

// ValidBgpEvents is the set of valid BGP event types.
// Includes all types accepted in config receive flags (base + directions).
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
	DirectionSent:     true, // "sent" — config receive flag for sent UPDATE events
}

// ValidRibEvents is the set of valid RIB event types.
var ValidRibEvents = map[string]bool{
	EventCache: true,
	EventRoute: true,
}

// ValidEvents maps namespace to its set of valid event types.
// This is the single source of truth for namespace/event validation.
// Protected by eventsMu for concurrent access.
var ValidEvents = map[string]map[string]bool{
	NamespaceBGP: ValidBgpEvents,
	NamespaceRIB: ValidRibEvents,
}

// IsValidEvent returns true if the event type is valid in the given namespace.
// Safe for concurrent use.
func IsValidEvent(namespace, eventType string) bool {
	eventsMu.RLock()
	defer eventsMu.RUnlock()
	events, ok := ValidEvents[namespace]
	if !ok {
		return false
	}
	return events[eventType]
}

// IsValidNamespace returns true if the namespace exists. Safe for concurrent use.
func IsValidNamespace(namespace string) bool {
	eventsMu.RLock()
	defer eventsMu.RUnlock()
	_, ok := ValidEvents[namespace]
	return ok
}

// ValidEventNames returns a sorted, comma-separated list of valid event types
// for the given namespace. Used in error messages so the list stays in sync
// with the registration maps. Safe for concurrent use.
func ValidEventNames(namespace string) string {
	eventsMu.RLock()
	defer eventsMu.RUnlock()
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
// Safe for concurrent use.
func ValidNamespaceNames() string {
	eventsMu.RLock()
	defer eventsMu.RUnlock()
	names := make([]string, 0, len(ValidEvents))
	for k := range ValidEvents {
		names = append(names, k)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

// IsValidEventAnyNamespace returns true if the event type is valid in any namespace.
// Safe for concurrent use.
func IsValidEventAnyNamespace(eventType string) bool {
	eventsMu.RLock()
	defer eventsMu.RUnlock()
	for _, events := range ValidEvents {
		if events[eventType] {
			return true
		}
	}
	return false
}

// AllEventTypes returns all valid event types grouped by namespace.
// Safe for concurrent use.
func AllEventTypes() map[string][]string {
	eventsMu.RLock()
	defer eventsMu.RUnlock()
	result := make(map[string][]string, len(ValidEvents))
	for ns, events := range ValidEvents {
		types := make([]string, 0, len(events))
		for et := range events {
			types = append(types, et)
		}
		result[ns] = types
	}
	return result
}

// AllValidEventNames returns a sorted, comma-separated list of valid event types
// across all namespaces (deduped). Safe for concurrent use.
func AllValidEventNames() string {
	eventsMu.RLock()
	defer eventsMu.RUnlock()
	seen := make(map[string]bool)
	for _, events := range ValidEvents {
		for k := range events {
			seen[k] = true
		}
	}
	names := make([]string, 0, len(seen))
	for k := range seen {
		names = append(names, k)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

// RegisterEventType adds a custom event type to the given namespace.
// Plugins call this to register event types they produce (e.g., "update-rpki").
// Duplicate registration is idempotent. The namespace must already exist.
// Event type names must be non-empty and contain no whitespace.
// Safe for concurrent use.
func RegisterEventType(namespace, eventType string) error {
	if eventType == "" {
		return fmt.Errorf("event type must not be empty")
	}
	if strings.ContainsAny(eventType, " \t\n\r") {
		return fmt.Errorf("event type %q must not contain whitespace", eventType)
	}
	eventsMu.Lock()
	defer eventsMu.Unlock()
	events, ok := ValidEvents[namespace]
	if !ok {
		return fmt.Errorf("unknown namespace: %s (valid: %s)", namespace, validNamespaceNamesLocked())
	}
	events[eventType] = true
	return nil
}

// validNamespaceNamesLocked returns namespace names. Caller MUST hold eventsMu.
func validNamespaceNamesLocked() string {
	names := make([]string, 0, len(ValidEvents))
	for k := range ValidEvents {
		names = append(names, k)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

// sendTypesMu protects ValidSendTypes from concurrent read/write.
// Writes happen during RegisterSendType (startup).
// Reads happen during parseOneSendFlag (config parsing).
var sendTypesMu sync.RWMutex

// ValidSendTypes is the set of plugin-registered send types.
// Base types (update, refresh) are handled by dedicated bool fields;
// this map holds only plugin-registered types (e.g., "enhanced-refresh").
// Protected by sendTypesMu for concurrent access.
var ValidSendTypes = map[string]bool{}

// RegisterSendType adds a plugin-registered send type to ValidSendTypes.
// Plugins call this to register send types they enable (e.g., "enhanced-refresh").
// Duplicate registration is idempotent.
// Send type names must be non-empty and contain no whitespace.
// Safe for concurrent use.
func RegisterSendType(sendType string) error {
	if sendType == "" {
		return fmt.Errorf("send type must not be empty")
	}
	if strings.ContainsAny(sendType, " \t\n\r") {
		return fmt.Errorf("send type %q must not contain whitespace", sendType)
	}
	sendTypesMu.Lock()
	defer sendTypesMu.Unlock()
	ValidSendTypes[sendType] = true
	return nil
}

// IsValidSendType returns true if the send type is a registered plugin send type.
// Safe for concurrent use.
func IsValidSendType(sendType string) bool {
	sendTypesMu.RLock()
	defer sendTypesMu.RUnlock()
	return ValidSendTypes[sendType]
}

// ValidSendTypeNames returns a sorted, comma-separated list of valid plugin-registered
// send types. Used in error messages. Safe for concurrent use.
func ValidSendTypeNames() string {
	sendTypesMu.RLock()
	defer sendTypesMu.RUnlock()
	if len(ValidSendTypes) == 0 {
		return ""
	}
	names := make([]string, 0, len(ValidSendTypes))
	for k := range ValidSendTypes {
		names = append(names, k)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}
