// Design: docs/architecture/api/process-protocol.md — plugin server configuration
// Related: startup_autoload.go — consumes ConfiguredFamilies, ConfiguredCustomEvents, ConfiguredCustomSendTypes
// Related: managed.go — hub-side managed config handlers

package server

import (
	plugin "codeberg.org/thomas-mangin/ze/internal/component/plugin"
	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
)

// ServerConfig holds API server configuration.
type ServerConfig struct {
	ConfigPath                string                // Path to config file (for peer save)
	Plugins                   []plugin.PluginConfig // External plugins to spawn
	ConfiguredFamilies        []string              // Families configured on peers (for deferred auto-load)
	ConfiguredCustomEvents    []string              // Custom event types in peer receive config (for auto-load)
	ConfiguredCustomSendTypes []string              // Custom send types in peer send config (for auto-load)
	ConfiguredPaths           []string              // Top-level config sections present (for config-driven auto-load)
	Hub                       *plugin.HubConfig     // TLS transport config (nil = no TLS listener)
	MetricsRegistry           metrics.Registry      // Prometheus metrics registry (nil = metrics disabled)
}
