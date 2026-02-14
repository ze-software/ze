// Package handler contains BGP-specific command handlers for the plugin server.
// These handlers implement BGP peer operations, cache management, commit workflow,
// raw message sending, and route refresh commands.
//
// Handlers are registered via BgpHandlerRPCs() which returns RPC registrations
// injected into the plugin server via ServerConfig.RPCProviders.
package handler

import (
	"codeberg.org/thomas-mangin/ze/internal/plugin"
)

// Command source constants (mirrored from plugin package for use in handler output).
const (
	sourceBuiltin = "builtin"
	argVerbose    = "verbose"
)

// BgpHandlerRPCs returns all RPC registrations from BGP handler files.
// This is injected into ServerConfig.RPCProviders by the reactor.
func BgpHandlerRPCs() []plugin.RPCRegistration {
	sources := [][]plugin.RPCRegistration{
		PeerOpsRPCs(),
		IntrospectionRPCs(),
		CacheRPCs(),
		CommitRPCs(),
		RawRPCs(),
		RefreshRPCs(),
	}
	n := 0
	for _, s := range sources {
		n += len(s)
	}
	rpcs := make([]plugin.RPCRegistration, 0, n)
	for _, s := range sources {
		rpcs = append(rpcs, s...)
	}
	return rpcs
}
