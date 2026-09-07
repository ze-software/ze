// Design: docs/architecture/api/process-protocol.md — plugin server configuration
// Related: startup_autoload.go — consumes ConfiguredFamilies, ConfiguredCustomEvents, ConfiguredCustomSendTypes
// Related: managed.go — hub-side managed config handlers

package server

import (
	plugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/metrics"
)

// ServerConfig holds API server configuration.
type ServerConfig struct {
	ConfigPath                string                // Path to config file (for peer save)
	Plugins                   []plugin.PluginConfig // External plugins to spawn
	ConfiguredFamilies        []string              // Families configured on peers (for deferred auto-load)
	ConfiguredCustomEvents    []string              // Custom event types in peer receive config (for auto-load)
	ConfiguredCustomSendTypes []string              // Custom send types in peer send config (for auto-load)
	ConfiguredPaths           []string              // Top-level config sections present (for config-driven auto-load)
	// DataPlane is the backend `interface { backend }` selects ("netlink",
	// "vpp"), read from the startup config tree. It answers which FIB plugin
	// writes for a route producer that declares Registration.NeedsDataPlane.
	// Empty means the tree named none and the build has no default, which is
	// the non-Linux case: no writer is resolved and the producer's doctor check
	// says so.
	DataPlane       string
	Hub             *plugin.HubConfig // TLS transport config (nil = no TLS listener)
	MetricsRegistry metrics.Registry  // Prometheus metrics registry (nil = metrics disabled)
}
