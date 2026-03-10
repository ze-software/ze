// Design: docs/architecture/api/commands.md — RIB CLI proxy handlers
//
// Package rib registers CLI proxy handlers that forward RIB commands to
// the bgp-rib plugin process. Each handler bridges the compile-time builtin
// RPC path (AllBuiltinRPCs → BuildCommandTree → ze show/run) to the runtime
// plugin command path (CommandRegistry → routeToProcess → bgp-rib SDK).
//
// Without these proxies, RIB commands are only reachable through the
// interactive CLI's plugin dispatch fallback, not through ze show/run.
package rib

import (
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

func init() {
	pluginserver.RegisterRPCs(
		// Read-only commands (exposed via "ze show")
		pluginserver.RPCRegistration{WireMethod: "ze-rib-api:status", CLICommand: "bgp rib status", Handler: forwardRibStatus, Help: "RIB summary (peer count, route counts)", ReadOnly: true},
		pluginserver.RPCRegistration{WireMethod: "ze-rib-api:routes", CLICommand: "bgp rib routes", Handler: forwardRibRoutes, Help: "Routes (scope + filters + terminal)", ReadOnly: true},
		pluginserver.RPCRegistration{WireMethod: "ze-rib-api:best", CLICommand: "bgp rib best", Handler: forwardRibBest, Help: "Best-path per prefix", ReadOnly: true},
		pluginserver.RPCRegistration{WireMethod: "ze-rib-api:best-status", CLICommand: "bgp rib best status", Handler: forwardRibBestStatus, Help: "Best-path computation status", ReadOnly: true},
		// Write commands (exposed via "ze run" only)
		pluginserver.RPCRegistration{WireMethod: "ze-rib-api:clear-in", CLICommand: "bgp rib clear in", Handler: forwardRibClearIn, Help: "Clear Adj-RIB-In routes"},
		pluginserver.RPCRegistration{WireMethod: "ze-rib-api:clear-out", CLICommand: "bgp rib clear out", Handler: forwardRibClearOut, Help: "Resend Adj-RIB-Out routes"},
	)
}

func forwardRibStatus(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return ctx.Dispatcher().ForwardToPlugin("rib status", args, ctx.PeerSelector())
}

func forwardRibRoutes(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return ctx.Dispatcher().ForwardToPlugin("rib show", args, ctx.PeerSelector())
}

func forwardRibBest(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return ctx.Dispatcher().ForwardToPlugin("rib best", args, ctx.PeerSelector())
}

func forwardRibBestStatus(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return ctx.Dispatcher().ForwardToPlugin("rib best status", args, ctx.PeerSelector())
}

func forwardRibClearIn(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return ctx.Dispatcher().ForwardToPlugin("rib clear in", args, ctx.PeerSelector())
}

func forwardRibClearOut(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return ctx.Dispatcher().ForwardToPlugin("rib clear out", args, ctx.PeerSelector())
}
