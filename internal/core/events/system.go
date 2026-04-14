// Design: docs/architecture/api/process-protocol.md -- system event types

package events

// System event types.
const (
	EventClockSynced = "clock-synced" // NTP plugin: system clock set from NTP
)

// ValidSystemEvents is the set of valid system-level event types.
var ValidSystemEvents = map[string]bool{
	EventClockSynced: true,
}
