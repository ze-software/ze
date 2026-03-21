// Design: docs/architecture/api/process-protocol.md — plugin server configuration
// Related: startup_autoload.go — consumes ConfiguredFamilies, ConfiguredCustomEvents, ConfiguredCustomSendTypes

package server

import (
	"encoding/json"

	plugin "codeberg.org/thomas-mangin/ze/internal/component/plugin"
)

// ServerConfig holds API server configuration.
type ServerConfig struct {
	ConfigPath                string                                          // Path to config file (for peer save)
	Plugins                   []plugin.PluginConfig                           // External plugins to spawn
	ConfiguredFamilies        []string                                        // Families configured on peers (for deferred auto-load)
	ConfiguredCustomEvents    []string                                        // Custom event types in peer receive config (for auto-load)
	ConfiguredCustomSendTypes []string                                        // Custom send types in peer send config (for auto-load)
	Hub                       *plugin.HubConfig                               // TLS transport config (nil = no TLS listener)
	RPCFallback               func(string) func(json.RawMessage) (any, error) // Resolves RPC methods not in core dispatch
	CommitManager             any                                             // Commit manager instance (injected by reactor, type-asserted by handlers)
}
