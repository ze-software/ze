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

// Plugin command names — used in both RPCRegistration.PluginCommand and ForwardToPlugin
// to prevent divergence between the two.
const (
	cmdRibStatus     = "rib status"
	cmdRibShow       = "rib show"
	cmdRibBest       = "rib best"
	cmdRibBestStatus = "rib best status"
	cmdRibClearIn    = "rib clear in"
	cmdRibClearOut   = "rib clear out"
)

func init() {
	pluginserver.RegisterRPCs(
		// Read-only commands (exposed via "ze show")
		pluginserver.RPCRegistration{WireMethod: "ze-rib-api:status", CLICommand: "bgp rib status", Handler: forwardRibStatus, Help: "RIB summary (peer count, route counts)", ReadOnly: true, PluginCommand: cmdRibStatus},
		pluginserver.RPCRegistration{WireMethod: "ze-rib-api:routes", CLICommand: "bgp rib routes", Handler: forwardRibRoutes, Help: "Routes (scope + filters + terminal)", ReadOnly: true, PluginCommand: cmdRibShow},
		pluginserver.RPCRegistration{WireMethod: "ze-rib-api:best", CLICommand: "bgp rib best", Handler: forwardRibBest, Help: "Best-path per prefix", ReadOnly: true, PluginCommand: cmdRibBest},
		pluginserver.RPCRegistration{WireMethod: "ze-rib-api:best-status", CLICommand: "bgp rib best status", Handler: forwardRibBestStatus, Help: "Best-path computation status", ReadOnly: true, PluginCommand: cmdRibBestStatus},
		// Write commands (exposed via "ze run" only)
		pluginserver.RPCRegistration{WireMethod: "ze-rib-api:clear-in", CLICommand: "bgp rib clear in", Handler: forwardRibClearIn, Help: "Clear Adj-RIB-In routes", PluginCommand: cmdRibClearIn},
		pluginserver.RPCRegistration{WireMethod: "ze-rib-api:clear-out", CLICommand: "bgp rib clear out", Handler: forwardRibClearOut, Help: "Resend Adj-RIB-Out routes", PluginCommand: cmdRibClearOut},
	)
}

func forwardRibStatus(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return ctx.Dispatcher().ForwardToPlugin(cmdRibStatus, args, ctx.PeerSelector())
}

func forwardRibRoutes(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return ctx.Dispatcher().ForwardToPlugin(cmdRibShow, args, ctx.PeerSelector())
}

func forwardRibBest(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return ctx.Dispatcher().ForwardToPlugin(cmdRibBest, args, ctx.PeerSelector())
}

func forwardRibBestStatus(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return ctx.Dispatcher().ForwardToPlugin(cmdRibBestStatus, args, ctx.PeerSelector())
}

func forwardRibClearIn(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return ctx.Dispatcher().ForwardToPlugin(cmdRibClearIn, args, ctx.PeerSelector())
}

func forwardRibClearOut(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return ctx.Dispatcher().ForwardToPlugin(cmdRibClearOut, args, ctx.PeerSelector())
}
