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
	NamespaceBGP       = "bgp"
	NamespaceRIB       = "rib"
	NamespaceConfig    = "config"
	NamespaceSysrib    = "sysrib"
	NamespaceFib       = "fib"
	NamespaceInterface = "interface"
)

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

// RIB event types.
const (
	EventCache         = "cache"
	EventRoute         = "route"
	EventBestChange    = "best-change"    // protocol RIB published a best-path change
	EventReplayRequest = "replay-request" // downstream consumer asking for full table replay
)

// Sysrib event types.
const (
	EventSysribBestChange    = "best-change"    // sysrib published a system-wide best change
	EventSysribReplayRequest = "replay-request" // downstream consumer asking sysrib to replay
)

// Fib event types.
const (
	EventFibExternalChange = "external-change" // FIB observed a route installed by something other than ze
)

// Interface event types.
const (
	EventInterfaceCreated      = "created"
	EventInterfaceUp           = "up"
	EventInterfaceDown         = "down"
	EventInterfaceAddrAdded    = "addr-added"
	EventInterfaceAddrRemoved  = "addr-removed"
	EventInterfaceDHCPAcquired = "dhcp-acquired"
	EventInterfaceDHCPRenewed  = "dhcp-renewed"
	EventInterfaceDHCPExpired  = "dhcp-expired"
	EventInterfaceRollback     = "rollback"
)

// Config transaction event types.
// Engine emits per-plugin verify/apply events. Plugins ack with broadcast events.
// See plan/spec-config-tx-protocol.md and docs/architecture/config/transaction-protocol.md.
const (
	EventConfigVerify       = "verify"        // Engine -> plugin: validate candidate (per-plugin variant: "verify-<plugin>")
	EventConfigApply        = "apply"         // Engine -> plugin: apply changes (per-plugin variant: "apply-<plugin>")
	EventConfigRollback     = "rollback"      // Engine -> plugins: undo changes
	EventConfigCommitted    = "committed"     // Engine -> plugins: discard journals
	EventConfigApplied      = "applied"       // Engine -> observers: transaction committed
	EventConfigRolledBack   = "rolled-back"   // Engine -> observers: transaction rolled back
	EventConfigVerifyAbort  = "verify-abort"  // Engine -> plugins: verify phase aborted
	EventConfigVerifyOK     = "verify-ok"     // Plugin -> engine: verification passed
	EventConfigVerifyFailed = "verify-failed" // Plugin -> engine: verification rejected
	EventConfigApplyOK      = "apply-ok"      // Plugin -> engine: apply succeeded
	EventConfigApplyFailed  = "apply-failed"  // Plugin -> engine: apply failed, trigger rollback
	EventConfigRollbackOK   = "rollback-ok"   // Plugin -> engine: rollback complete
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
	EventListenerReady:      true, // BGP TCP listener bound and accepting
	EventUpdateNotification: true, // Lightweight observability notification for UPDATE arrivals/sends
	DirectionSent:           true, // "sent" — config receive flag for sent UPDATE events
}

// ValidRibEvents is the set of valid RIB event types.
var ValidRibEvents = map[string]bool{
	EventCache:         true,
	EventRoute:         true,
	EventBestChange:    true, // protocol RIB published a best-path change
	EventReplayRequest: true, // downstream consumer asking for full table replay
}

// ValidSysribEvents is the set of valid sysrib event types.
var ValidSysribEvents = map[string]bool{
	EventSysribBestChange:    true,
	EventSysribReplayRequest: true,
}

// ValidFibEvents is the set of valid FIB event types.
var ValidFibEvents = map[string]bool{
	EventFibExternalChange: true,
}

// ValidInterfaceEvents is the set of valid interface monitor event types.
var ValidInterfaceEvents = map[string]bool{
	EventInterfaceCreated:      true,
	EventInterfaceUp:           true,
	EventInterfaceDown:         true,
	EventInterfaceAddrAdded:    true,
	EventInterfaceAddrRemoved:  true,
	EventInterfaceDHCPAcquired: true,
	EventInterfaceDHCPRenewed:  true,
	EventInterfaceDHCPExpired:  true,
	EventInterfaceRollback:     true,
}

// ValidConfigEvents is the set of valid config transaction event types.
// Per-plugin variants ("verify-<plugin>", "apply-<plugin>") are registered
// dynamically as plugins start, via RegisterEventType(NamespaceConfig, ...).
var ValidConfigEvents = map[string]bool{
	EventConfigVerify:       true,
	EventConfigApply:        true,
	EventConfigRollback:     true,
	EventConfigCommitted:    true,
	EventConfigApplied:      true,
	EventConfigRolledBack:   true,
	EventConfigVerifyAbort:  true,
	EventConfigVerifyOK:     true,
	EventConfigVerifyFailed: true,
	EventConfigApplyOK:      true,
	EventConfigApplyFailed:  true,
	EventConfigRollbackOK:   true,
}

// ValidEvents maps namespace to its set of valid event types.
// This is the single source of truth for namespace/event validation.
// Protected by eventsMu for concurrent access.
var ValidEvents = map[string]map[string]bool{
	NamespaceBGP:       ValidBgpEvents,
	NamespaceRIB:       ValidRibEvents,
	NamespaceConfig:    ValidConfigEvents,
	NamespaceSysrib:    ValidSysribEvents,
	NamespaceFib:       ValidFibEvents,
	NamespaceInterface: ValidInterfaceEvents,
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
