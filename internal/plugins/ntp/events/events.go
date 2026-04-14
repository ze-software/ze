// Design: docs/architecture/api/process-protocol.md -- system event types

// Package events defines event constants for the NTP/system plugin.
// This is a leaf package with no dependencies, safe to import from anywhere.
package events

// Namespace is the event namespace for system-level events.
const Namespace = "system"

// System event types.
const (
	EventClockSynced = "clock-synced" // NTP plugin: system clock set from NTP
)
