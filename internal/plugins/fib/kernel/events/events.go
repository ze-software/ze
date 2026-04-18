// Design: docs/architecture/api/process-protocol.md -- FIB event types

// Package events defines event constants for the FIB kernel plugin.
// This is a leaf package with no dependencies, safe to import from anywhere.
package events

// Namespace is the event namespace for the FIB kernel plugin.
const Namespace = "fib"

// FIB event types.
const (
	EventExternalChange = "external-change" // FIB observed a route installed by something other than ze
)
