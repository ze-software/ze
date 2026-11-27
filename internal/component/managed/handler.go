// Design: docs/architecture/fleet-config.md — managed client RPC handlers
// Related: client.go — connection lifecycle uses Handler for config processing

package managed

import (
	"encoding/base64"

	"github.com/ze-software/ze/pkg/fleet"
)

// Handler processes managed config RPCs on the client side.
// Callbacks are injected by the client component that owns the blob store
// and config reload logic.
type Handler struct {
	// OnFetch is called when a config-changed notification arrives.
	// The version parameter is the new config's version hash.
	// If nil, notifications are ignored.
	OnFetch func(version string)

	// Validate checks whether raw config bytes are valid.
	// Returns nil if the config is acceptable.
	Validate func(data []byte) error

	// Cache is deprecated. Fetched config is now committed through ClientConfig.CommitOver.
	Cache func(data []byte) error
}

// handleConfigChanged processes a config-changed notification from the hub.
// Triggers OnFetch if set.
func (h *Handler) handleConfigChanged(n fleet.ConfigChanged) {
	if h.OnFetch != nil {
		h.OnFetch(n.Version)
	}
}

// ProcessConfig validates a config received from the hub.
// Returns a ConfigAck indicating success or failure.
func (h *Handler) ProcessConfig(resp fleet.ConfigFetchResponse) fleet.ConfigAck {
	_, ack := h.ValidateConfig(resp)
	return ack
}

// ValidateRawConfig runs the configured full YANG validation callback against
// raw configuration bytes (used for both full configs and diff-reconstructed
// configs before staging).
func (h *Handler) ValidateRawConfig(data []byte) error {
	if h == nil {
		return nil
	}
	if h.Validate == nil {
		return nil
	}
	return h.Validate(data)
}

// ValidateConfig decodes and validates a config received from the hub.
func (h *Handler) ValidateConfig(resp fleet.ConfigFetchResponse) ([]byte, fleet.ConfigAck) {
	data, err := base64.StdEncoding.DecodeString(resp.Config)
	if err != nil {
		return nil, fleet.ConfigAck{
			Version:    resp.Version,
			OK:         false,
			Error:      "decode config: " + err.Error(),
			RejectKind: fleet.RejectKindResync,
		}
	}

	if err := h.ValidateRawConfig(data); err != nil {
		return nil, fleet.ConfigAck{
			Version:    resp.Version,
			OK:         false,
			Error:      "config failed validation",
			RejectKind: fleet.RejectKindVerify,
		}
	}

	return data, fleet.ConfigAck{
		Version: resp.Version,
		OK:      true,
	}
}
