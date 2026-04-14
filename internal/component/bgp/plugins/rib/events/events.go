// Design: docs/architecture/api/process-protocol.md -- BGP-RIB event types

// Package events defines event constants for the BGP RIB plugin.
// This is a leaf package with no dependencies, safe to import from anywhere.
package events

// Namespace is the event namespace for the BGP RIB plugin.
const Namespace = "bgp-rib"

// RIB event types.
const (
	EventCache         = "cache"
	EventRoute         = "route"
	EventBestChange    = "best-change"    // protocol RIB published a best-path change
	EventReplayRequest = "replay-request" // downstream consumer asking for full table replay
)
