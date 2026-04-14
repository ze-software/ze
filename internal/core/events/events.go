// Design: docs/architecture/api/process-protocol.md -- event bus namespace and type registry
//
// Package events defines the event namespace and type constants used by ze's
// event bus, plus the runtime registry for namespace/event validation.
// Domain-specific event types are defined in separate files (bgp.go,
// interface.go, sysctl.go, etc.) so ownership is clear at the file level.
// Each domain file declares its constants and valid-event map; the aggregate
// ValidEvents map is assembled here from those per-domain maps.
package events

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

// Event namespaces.
const (
	NamespaceBGP       = "bgp"
	NamespaceBGPRIB    = "bgp-rib"
	NamespaceConfig    = "config"
	NamespaceSystemRIB = "system-rib"
	NamespaceFib       = "fib"
	NamespaceInterface = "interface"
	NamespaceSysctl    = "sysctl"
	NamespaceSystem    = "system"
	NamespaceVPP       = "vpp"
)

// Direction constants for event filtering.
const (
	DirectionReceived = "received"
	DirectionSent     = "sent"
	DirectionBoth     = "both"
)

// eventsMu protects ValidEvents from concurrent read/write.
// Writes happen during RegisterEventType and RegisterNamespace (startup).
// Reads happen during subscribe-events and emit-event validation (runtime).
var eventsMu sync.RWMutex

// ValidEvents maps namespace to its set of valid event types.
// This is the single source of truth for namespace/event validation.
// Protected by eventsMu for concurrent access.
var ValidEvents = map[string]map[string]bool{
	NamespaceBGP:       ValidBgpEvents,
	NamespaceBGPRIB:    ValidBGPRIBEvents,
	NamespaceConfig:    ValidConfigEvents,
	NamespaceSystemRIB: ValidSystemRIBEvents,
	NamespaceFib:       ValidFibEvents,
	NamespaceInterface: ValidInterfaceEvents,
	NamespaceSysctl:    ValidSysctlEvents,
	NamespaceSystem:    ValidSystemEvents,
	NamespaceVPP:       ValidVPPEvents,
}

// IsValidEvent returns true if the event type is valid in the given namespace.
// Safe for concurrent use.
func IsValidEvent(namespace, eventType string) bool {
	eventsMu.RLock()
	defer eventsMu.RUnlock()
	evts, ok := ValidEvents[namespace]
	if !ok {
		return false
	}
	return evts[eventType]
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
	for _, evts := range ValidEvents {
		if evts[eventType] {
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
	for ns, evts := range ValidEvents {
		types := make([]string, 0, len(evts))
		for et := range evts {
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
	for _, evts := range ValidEvents {
		for k := range evts {
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

// RegisterNamespace adds a new namespace with the given initial event types.
// The namespace must not already exist. Use RegisterEventType to add events
// to an existing namespace. Safe for concurrent use.
func RegisterNamespace(namespace string, eventTypes ...string) error {
	if namespace == "" {
		return fmt.Errorf("namespace must not be empty")
	}
	eventsMu.Lock()
	defer eventsMu.Unlock()
	if _, exists := ValidEvents[namespace]; exists {
		return fmt.Errorf("namespace %q already registered", namespace)
	}
	m := make(map[string]bool, len(eventTypes))
	for _, e := range eventTypes {
		m[e] = true
	}
	ValidEvents[namespace] = m
	return nil
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
	evts, ok := ValidEvents[namespace]
	if !ok {
		return fmt.Errorf("unknown namespace: %s (valid: %s)", namespace, validNamespaceNamesLocked())
	}
	evts[eventType] = true
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
