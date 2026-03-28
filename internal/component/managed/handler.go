// Design: docs/architecture/fleet-config.md — managed client RPC handlers
// Related: client.go — connection lifecycle uses Handler for config processing

package managed

import (
	"encoding/base64"
	"fmt"

	"codeberg.org/thomas-mangin/ze/pkg/fleet"
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

	// Cache writes validated config bytes to the local blob store.
	Cache func(data []byte) error
}

// HandleConfigChanged processes a config-changed notification from the hub.
// Triggers OnFetch if set.
func (h *Handler) HandleConfigChanged(n fleet.ConfigChanged) {
	if h.OnFetch != nil {
		h.OnFetch(n.Version)
	}
}

// ProcessConfig validates and caches a config received from the hub.
// Returns a ConfigAck indicating success or failure.
func (h *Handler) ProcessConfig(resp fleet.ConfigFetchResponse) fleet.ConfigAck {
	data, err := base64.StdEncoding.DecodeString(resp.Config)
	if err != nil {
		return fleet.ConfigAck{
			Version: resp.Version,
			OK:      false,
			Error:   fmt.Sprintf("decode config: %v", err),
		}
	}

	if h.Validate != nil {
		if err := h.Validate(data); err != nil {
			return fleet.ConfigAck{
				Version: resp.Version,
				OK:      false,
				Error:   fmt.Sprintf("validate config: %v", err),
			}
		}
	}

	if h.Cache != nil {
		if err := h.Cache(data); err != nil {
			return fleet.ConfigAck{
				Version: resp.Version,
				OK:      false,
				Error:   fmt.Sprintf("cache config: %v", err),
			}
		}
	}

	return fleet.ConfigAck{
		Version: resp.Version,
		OK:      true,
	}
}
