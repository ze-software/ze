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
	"codeberg.org/thomas-mangin/ze/internal/component/command"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

// Plugin command names — used in both RPCRegistration.PluginCommand and ForwardToPlugin
// to prevent divergence between the two.
const (
	cmdRibStatus     = "bgp rib status"
	cmdRibShow       = "bgp rib show"
	cmdRibBest       = "bgp rib show best"
	cmdRibBestStatus = "bgp rib show best status"
	cmdRibClearIn    = "bgp rib clear in"
	cmdRibClearOut   = "bgp rib clear out"
	cmdRibInject     = "bgp rib inject"
	cmdRibWithdraw   = "bgp rib withdraw"
	cmdRibRPF        = "bgp rib rpf"
)

func init() {
	registerPipeFilters()

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
		pluginserver.RPCRegistration{WireMethod: "ze-rib-api:rpf", Handler: forwardRibRPF, PluginCommand: cmdRibRPF},
	)
}

func registerPipeFilters() {
	command.RegisterPipeFilters([]string{"show bgp rib", "rib routes", cmdRibShow},
		command.PipeFilter{Name: "received", Description: "Select received routes", Leading: true},
		command.PipeFilter{Name: "advertised", Description: "Select advertised routes", Leading: true},
		command.PipeFilter{Name: "peer", Description: "Filter by peer", TakesArg: true},
		command.PipeFilter{Name: "family", Description: "Filter by AFI/SAFI", TakesArg: true},
		command.PipeFilter{Name: "prefix", Description: "Filter by prefix", TakesArg: true},
		command.PipeFilter{Name: "path", Description: "Filter by AS path", TakesArg: true},
		command.PipeFilter{Name: "community", Description: "Filter by standard community", TakesArg: true},
		command.PipeFilter{Name: "match", Description: "Cross-field structured match", TakesArg: true},
		command.PipeFilter{Name: "count", Description: "Count matching routes without serializing rows"},
		command.PipeFilter{Name: "prefix-summary", Description: "Summarize by family and prefix length"},
		command.PipeFilter{Name: "graph", Description: "Render AS-path topology graph"},
	)
	command.RegisterPipeFilters([]string{"show bgp rib best", "rib best", cmdRibBest},
		command.PipeFilter{Name: "peer", Description: "Filter by peer", TakesArg: true},
		command.PipeFilter{Name: "family", Description: "Filter by AFI/SAFI", TakesArg: true},
		command.PipeFilter{Name: "prefix", Description: "Filter by prefix", TakesArg: true},
		command.PipeFilter{Name: "path", Description: "Filter by AS path", TakesArg: true},
		command.PipeFilter{Name: "community", Description: "Filter by standard community", TakesArg: true},
		command.PipeFilter{Name: "match", Description: "Cross-field structured match", TakesArg: true},
		command.PipeFilter{Name: "count", Description: "Count matching best paths without serializing rows"},
		command.PipeFilter{Name: "prefix-summary", Description: "Summarize by family and prefix length"},
		command.PipeFilter{Name: "graph", Description: "Render AS-path topology graph"},
		command.PipeFilter{Name: "reason", Description: "Explain best-path selection"},
	)
	command.RegisterPipeFilters([]string{"show bgp rib status", "show bgp rib best status", "rib status", "rib best status", cmdRibStatus, cmdRibBestStatus})
}

func forwardRibStatus(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return ctx.Dispatcher().ForwardToPlugin(ctx, cmdRibStatus, args, ctx.PeerSelector())
}

func forwardRibRoutes(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return ctx.Dispatcher().ForwardToPlugin(ctx, cmdRibShow, args, ctx.PeerSelector())
}

func forwardRibBest(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return ctx.Dispatcher().ForwardToPlugin(ctx, cmdRibBest, args, ctx.PeerSelector())
}

func forwardRibBestStatus(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return ctx.Dispatcher().ForwardToPlugin(ctx, cmdRibBestStatus, args, ctx.PeerSelector())
}

func forwardRibClearIn(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return ctx.Dispatcher().ForwardToPlugin(ctx, cmdRibClearIn, args, ctx.PeerSelector())
}

func forwardRibClearOut(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return ctx.Dispatcher().ForwardToPlugin(ctx, cmdRibClearOut, args, ctx.PeerSelector())
}

func forwardRibInject(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return ctx.Dispatcher().ForwardToPlugin(ctx, cmdRibInject, args, ctx.PeerSelector())
}

func forwardRibWithdraw(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return ctx.Dispatcher().ForwardToPlugin(ctx, cmdRibWithdraw, args, ctx.PeerSelector())
}

func forwardRibRPF(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return ctx.Dispatcher().ForwardToPlugin(ctx, cmdRibRPF, args, ctx.PeerSelector())
}
