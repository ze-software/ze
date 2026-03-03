// Design: docs/architecture/api/process-protocol.md — plugin server configuration

package server

import (
	"encoding/json"

	plugin "codeberg.org/thomas-mangin/ze/internal/component/plugin"
)

// ServerConfig holds API server configuration.
type ServerConfig struct {
	SocketPath         string                                          // Path to Unix socket
	Plugins            []plugin.PluginConfig                           // External plugins to spawn
	ConfiguredFamilies []string                                        // Families configured on peers (for deferred auto-load)
	RPCProviders       []func() []RPCRegistration                      // Additional RPC sources (e.g., BGP handler RPCs)
	RPCFallback        func(string) func(json.RawMessage) (any, error) // Resolves RPC methods not in core dispatch
	CommitManager      any                                             // Commit manager instance (injected by reactor, type-asserted by handlers)
}
