// Design: docs/architecture/api/process-protocol.md -- system event types

// Package events defines event constants for the "system" event namespace.
// Owned by the NTP plugin (the sole producer of system-level events).
// If a second component needs system events, move this package to
// internal/core/system/events/ so the namespace is not coupled to NTP.
// This is a leaf package with no dependencies, safe to import from anywhere.
package events

// Namespace is the event namespace for system-level events.
// Owned by the NTP plugin; consumers import this package to subscribe.
const Namespace = "system"

// System event types.
const (
	EventClockSynced = "clock-synced" // NTP plugin: system clock set from NTP
)
