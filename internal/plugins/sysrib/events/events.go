// Design: docs/architecture/api/process-protocol.md -- system-RIB event types

// Package events defines event constants for the system RIB plugin.
// This is a leaf package with no dependencies, safe to import from anywhere.
package events

// Namespace is the event namespace for the system RIB plugin.
const Namespace = "system-rib"

// System-RIB event types.
const (
	EventBestChange    = "best-change"    // sysrib published a system-wide best change
	EventReplayRequest = "replay-request" // downstream consumer asking sysrib to replay
)
