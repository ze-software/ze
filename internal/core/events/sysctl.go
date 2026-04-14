// Design: docs/architecture/api/process-protocol.md -- sysctl event types

package events

// Sysctl event types.
const (
	EventSysctlDefault              = "default"                // Plugin -> sysctl: register a required default value
	EventSysctlSet                  = "set"                    // CLI -> sysctl: write a transient value
	EventSysctlApplied              = "applied"                // sysctl -> any: value was written to kernel
	EventSysctlShowRequest          = "show-request"           // CLI -> sysctl: request active keys table
	EventSysctlShowResult           = "show-result"            // sysctl -> requester: active keys JSON table
	EventSysctlListRequest          = "list-request"           // CLI -> sysctl: request known keys table
	EventSysctlListResult           = "list-result"            // sysctl -> requester: known keys JSON table
	EventSysctlDescribeRequest      = "describe-request"       // CLI -> sysctl: request detail for one key
	EventSysctlDescribeResult       = "describe-result"        // sysctl -> requester: key detail JSON
	EventSysctlClearProfileDefaults = "clear-profile-defaults" // iface -> sysctl: clear all profile defaults for an interface before re-emission
)

// ValidSysctlEvents is the set of valid sysctl event types.
var ValidSysctlEvents = map[string]bool{
	EventSysctlDefault:              true,
	EventSysctlSet:                  true,
	EventSysctlApplied:              true,
	EventSysctlShowRequest:          true,
	EventSysctlShowResult:           true,
	EventSysctlListRequest:          true,
	EventSysctlListResult:           true,
	EventSysctlDescribeRequest:      true,
	EventSysctlDescribeResult:       true,
	EventSysctlClearProfileDefaults: true,
}
