// Design: docs/architecture/api/process-protocol.md -- FIB event types

package events

// FIB event types.
const (
	EventFibExternalChange = "external-change" // FIB observed a route installed by something other than ze
)

// ValidFibEvents is the set of valid FIB event types.
var ValidFibEvents = map[string]bool{
	EventFibExternalChange: true,
}
