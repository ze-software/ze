// Design: docs/architecture/api/process-protocol.md -- RIB and system-RIB event types

package events

// RIB event types.
const (
	EventCache         = "cache"
	EventRoute         = "route"
	EventBestChange    = "best-change"    // protocol RIB published a best-path change
	EventReplayRequest = "replay-request" // downstream consumer asking for full table replay
)

// ValidBGPRIBEvents is the set of valid RIB event types.
var ValidBGPRIBEvents = map[string]bool{
	EventCache:         true,
	EventRoute:         true,
	EventBestChange:    true,
	EventReplayRequest: true,
}

// System-RIB event types.
const (
	EventSystemRIBBestChange    = "best-change"    // sysrib published a system-wide best change
	EventSystemRIBReplayRequest = "replay-request" // downstream consumer asking sysrib to replay
)

// ValidSystemRIBEvents is the set of valid sysrib event types.
var ValidSystemRIBEvents = map[string]bool{
	EventSystemRIBBestChange:    true,
	EventSystemRIBReplayRequest: true,
}
