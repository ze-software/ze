// Design: docs/architecture/api/process-protocol.md -- sysctl event types

// Package events defines event constants for the sysctl plugin.
// This is a leaf package with no dependencies, safe to import from anywhere.
package events

// Namespace is the event namespace for the sysctl plugin.
const Namespace = "sysctl"

// Sysctl event types.
const (
	EventDefault              = "default"                // Plugin -> sysctl: register a required default value
	EventSet                  = "set"                    // CLI -> sysctl: write a transient value
	EventApplied              = "applied"                // sysctl -> any: value was written to kernel
	EventShowRequest          = "show-request"           // CLI -> sysctl: request active keys table
	EventShowResult           = "show-result"            // sysctl -> requester: active keys JSON table
	EventListRequest          = "list-request"           // CLI -> sysctl: request known keys table
	EventListResult           = "list-result"            // sysctl -> requester: known keys JSON table
	EventDescribeRequest      = "describe-request"       // CLI -> sysctl: request detail for one key
	EventDescribeResult       = "describe-result"        // sysctl -> requester: key detail JSON
	EventClearProfileDefaults = "clear-profile-defaults" // iface -> sysctl: clear all profile defaults
)
