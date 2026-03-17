// Design: docs/architecture/api/process-protocol.md — plugin server configuration

package server

import (
	"encoding/json"

	"codeberg.org/thomas-mangin/ze/internal/component/config/yang"
	plugin "codeberg.org/thomas-mangin/ze/internal/component/plugin"
)

// ServerConfig holds API server configuration.
type ServerConfig struct {
	ConfigPath         string                                          // Path to config file (for peer save)
	Loader             *yang.Loader                                    // YANG loader for command tree (WireMethod -> CLI path)
	Plugins            []plugin.PluginConfig                           // External plugins to spawn
	ConfiguredFamilies []string                                        // Families configured on peers (for deferred auto-load)
	RPCFallback        func(string) func(json.RawMessage) (any, error) // Resolves RPC methods not in core dispatch
	CommitManager      any                                             // Commit manager instance (injected by reactor, type-asserted by handlers)
}
