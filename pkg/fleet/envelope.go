// Design: docs/architecture/fleet-config.md — managed config RPC payloads
// Related: version.go — version hash computation

package fleet

// RPC verb constants for managed config protocol.
const (
	VerbConfigFetch   = "config-fetch"
	VerbConfigChanged = "config-changed"
	VerbConfigAck     = "config-ack"
	VerbPing          = "ping"
)

// ConfigFetchRequest is sent by a managed client to request its config.
// Version is the client's current config hash, or empty on first boot.
type ConfigFetchRequest struct {
	Version string `json:"version"`
}

// ConfigFetchResponse is the hub's reply to a config-fetch request.
// If the client's version matches, Status is "current" and Config is empty.
// Otherwise, Version is the new hash and Config is base64-encoded config bytes.
type ConfigFetchResponse struct {
	Version string `json:"version,omitempty"`
	Config  string `json:"config,omitempty"`
	Status  string `json:"status,omitempty"`
}

// ConfigChanged is sent by the hub to notify a connected client
// that a new config version is available.
type ConfigChanged struct {
	Version string `json:"version"`
}

// ConfigAck is sent by the client after receiving and processing a config.
// OK is true if the config was accepted, false if validation failed.
// Error describes the rejection reason when OK is false.
type ConfigAck struct {
	Version string `json:"version"`
	OK      bool   `json:"ok"`
	Error   string `json:"error,omitempty"`
}
