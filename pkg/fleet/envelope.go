// Design: docs/architecture/fleet-config.md — managed config RPC payloads
// Related: version.go — version hash computation
// Related: report.go — inventory and health report payloads and producer registry

package fleet

import (
	"time"
	"unicode/utf8"
)

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
	// VerbInventoryReport is sent by a managed client to push its bounded
	// hardware and software summary to its hub.
	VerbInventoryReport = "inventory-report"
	// VerbHealthReport is sent by a managed client to push its component health
	// statuses to its hub.
	VerbHealthReport = "health-report"
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

// AckErrorMax is the maximum number of UTF-8 bytes a managed client sends in
// ConfigAck.Error. Longer messages are cut on a UTF-8 rune boundary and marked
// with AckErrorTruncatedSuffix so every legal client request stays well below
// the hub's managed read maximum.
const AckErrorMax = 1024

// AckErrorTruncatedSuffix marks a ConfigAck.Error value that was shortened to
// fit AckErrorMax.
const AckErrorTruncatedSuffix = "... (truncated)"

// BoundAckError shortens text to at most AckErrorMax bytes on a UTF-8 rune
// boundary and appends AckErrorTruncatedSuffix when truncated.
func BoundAckError(text string) string {
	if len(text) <= AckErrorMax {
		return text
	}
	limit := AckErrorMax - len(AckErrorTruncatedSuffix)
	for limit > 0 && !utf8.RuneStart(text[limit]) {
		limit--
	}
	return text[:limit] + AckErrorTruncatedSuffix
}

// HeldRefusedCode is the error code of a hub that takes config-held and
// refused this one copy: too large, a hash that is not the hash of the bytes,
// or a configuration that does not parse. A client then sends no copy of that
// version again, and may send a copy of another one. An error answer with any
// other code, or none, is a hub that does not take config-held at all: a hub
// built before the verb answers it as an unknown method.
const HeldRefusedCode = "config-held-refused"
