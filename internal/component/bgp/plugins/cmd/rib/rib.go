// Design: docs/architecture/api/commands.md — RIB CLI proxy handlers
// Overview: doc.go — bgp-cmd-rib plugin registration
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
	cmdRibBest       = "rib show best"
	cmdRibBestStatus = "rib show best status"
	cmdRibClearIn    = "rib clear in"
	cmdRibClearOut   = "rib clear out"
	cmdRibInject     = "rib inject"
	cmdRibWithdraw   = "rib withdraw"
)

func init() {
	pluginserver.RegisterRPCs(
		// Read-only commands (exposed via "ze show")
		pluginserver.RPCRegistration{WireMethod: "ze-rib-api:status", Handler: forwardRibStatus, PluginCommand: cmdRibStatus},
		pluginserver.RPCRegistration{WireMethod: "ze-rib-api:routes", Handler: forwardRibRoutes, PluginCommand: cmdRibShow},
		pluginserver.RPCRegistration{WireMethod: "ze-rib-api:best", Handler: forwardRibBest, PluginCommand: cmdRibBest},
		pluginserver.RPCRegistration{WireMethod: "ze-rib-api:best-status", Handler: forwardRibBestStatus, PluginCommand: cmdRibBestStatus},
		// Write commands (exposed via "ze run" only)
		pluginserver.RPCRegistration{WireMethod: "ze-rib-api:clear-in", Handler: forwardRibClearIn, PluginCommand: cmdRibClearIn},
		pluginserver.RPCRegistration{WireMethod: "ze-rib-api:clear-out", Handler: forwardRibClearOut, PluginCommand: cmdRibClearOut},
		pluginserver.RPCRegistration{WireMethod: "ze-rib-api:inject", Handler: forwardRibInject, PluginCommand: cmdRibInject},
		pluginserver.RPCRegistration{WireMethod: "ze-rib-api:withdraw", Handler: forwardRibWithdraw, PluginCommand: cmdRibWithdraw},
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

func forwardRibInject(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return ctx.Dispatcher().ForwardToPlugin(cmdRibInject, args, ctx.PeerSelector())
}

func forwardRibWithdraw(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return ctx.Dispatcher().ForwardToPlugin(cmdRibWithdraw, args, ctx.PeerSelector())
}
