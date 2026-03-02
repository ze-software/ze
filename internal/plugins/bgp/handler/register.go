// Package handler contains BGP-specific command handlers for the plugin server.
// These handlers implement BGP peer operations, cache management, commit workflow,
// raw message sending, route refresh commands, and RIB meta-commands.
//
// Handlers are registered via BgpHandlerRPCs() which returns RPC registrations
// injected into the plugin server via ServerConfig.RPCProviders.
//
// Detail: bgp.go — BGP introspection and peer operation handlers
// Detail: rib_meta.go — RIB meta-command handlers
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
		UpdateRPCs(),
		RibMetaRPCs(),
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
