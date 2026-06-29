// Design: docs/architecture/api/process-protocol.md -- VPP lifecycle event types

// Package events defines event constants for the VPP component.
// This is a leaf package with no dependencies, safe to import from anywhere.
package events

// Namespace is the event namespace for the VPP component.
const Namespace = "vpp"

// VPP lifecycle event types.
const (
	EventConnected    = "connected"    // VPP component -> dependents: GoVPP connection established
	EventDisconnected = "disconnected" // VPP component -> dependents: GoVPP connection lost
	EventReconnected  = "reconnected"  // VPP component -> dependents: GoVPP reconnected after crash
)
