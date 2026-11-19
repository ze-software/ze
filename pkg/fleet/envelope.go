// Design: docs/architecture/fleet-config.md — managed config RPC payloads
// Related: version.go — version hash computation

package fleet

import "time"

// RPC verb constants for managed config protocol.
const (
	VerbConfigFetch   = "config-fetch"
	VerbConfigChanged = "config-changed"
	VerbConfigAck     = "config-ack"
	// VerbConfigHeld is sent by a client that holds: it carries the
	// configuration the client runs up to its hub. A hub built before the verb
	// existed answers it with an unknown-method error.
	VerbConfigHeld = "config-held"
	VerbPing       = "ping"
)

// ConfigFetchRequest is sent by a managed client to request its config.
// Version is the client's current config hash, or empty on first boot.
type ConfigFetchRequest struct {
	Version string `json:"version"`
}

// ConfigFetchResponse is the hub's reply to a config-fetch request.
// If the client's version matches, Status is "current" and Config is empty.
// Otherwise, Version is the new hash and Config is base64-encoded config bytes.
//
// HeldVersion and DiscardVersion are optional, so a client built before them
// decodes the answer as before and ignores both. HeldVersion is the version
// hash of the configuration the hub holds as this client's held copy, and is
// absent when the hub holds none. DiscardVersion is present only when the
// hub's operator decided that a client running exactly that version drops it
// and applies Config.
type ConfigFetchResponse struct {
	Version        string `json:"version,omitempty"`
	Config         string `json:"config,omitempty"`
	Status         string `json:"status,omitempty"`
	HeldVersion    string `json:"held-version,omitempty"`
	DiscardVersion string `json:"discard-version,omitempty"`
}

// ConfigChanged is sent by the hub to notify a connected client
// that a new config version is available.
type ConfigChanged struct {
	Version string `json:"version"`
}

// ConfigAck is sent by the client after receiving and processing a config.
// OK is true if the config was accepted, false if validation failed.
// Error describes the rejection reason when OK is false.
//
// Holding is optional: true says the client refused because it holds a
// configuration of its own, not because the hub's one is invalid. A hub built
// before the field ignores it and reads Error as before.
type ConfigAck struct {
	Version string `json:"version"`
	OK      bool   `json:"ok"`
	Error   string `json:"error,omitempty"`
	Holding bool   `json:"holding,omitempty"`
}

// ConfigHeld is the payload of config-held: the configuration a holding client
// runs. It names no client: the hub takes the name from the authenticated
// session. Version is the version hash of the decoded Config bytes, which the
// hub recomputes and never trusts. Config is the base64-encoded configuration.
// EditedAt is when that configuration became active on the client, and is
// absent when the client's store records no such time.
type ConfigHeld struct {
	Version  string    `json:"version"`
	Config   string    `json:"config"`
	EditedAt time.Time `json:"edited-at,omitzero"`
}

// HeldConfigMax is the largest configuration, in raw bytes before base64, that
// a client sends in config-held and that a hub keeps. A client sends no
// larger one, and a hub refuses a larger one whatever the client claims. The
// bound is set by the client's notification loop, which answers no
// config-changed while an upload runs, not by the store.
const HeldConfigMax = 1 << 20

// HeldRefusedCode is the error code of a hub that takes config-held and
// refused this one copy: too large, a hash that is not the hash of the bytes,
// or a configuration that does not parse. A client then sends no copy of that
// version again, and may send a copy of another one. An error answer with any
// other code, or none, is a hub that does not take config-held at all: a hub
// built before the verb answers it as an unknown method.
const HeldRefusedCode = "config-held-refused"
