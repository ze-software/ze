// Design: docs/architecture/api/process-protocol.md -- VPP lifecycle event types

package events

// VPP lifecycle event types.
const (
	EventVPPConnected    = "connected"    // VPP component -> dependents: GoVPP connection established
	EventVPPDisconnected = "disconnected" // VPP component -> dependents: GoVPP connection lost
	EventVPPReconnected  = "reconnected"  // VPP component -> dependents: GoVPP reconnected after crash
)

// ValidVPPEvents is the set of valid VPP lifecycle event types.
var ValidVPPEvents = map[string]bool{
	EventVPPConnected:    true,
	EventVPPDisconnected: true,
	EventVPPReconnected:  true,
}
