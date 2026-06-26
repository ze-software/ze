// Design: plan/learned/967-ospf-13-cli-diag-interop.md -- `show ospf ...` under the
// top-level show grammar.
// Related: register.go -- runOSPFEngine registers the matching CommandDecl set and the
// OnExecuteCommand render dispatch.
// Related: yang/ze-ospf-cmd.yang -- the owner command tree binding these wire methods.
//
// The OSPF introspection data lives in the engine plugin process, reachable via the
// commands the component registers ("show ospf neighbor", ...). These builtin RPCs
// are plugin proxies (the LDP/IS-IS model): each declares the plugin command it fronts
// via PluginCommand so the OSPF engine can register the same command name (instead of a
// builtin conflict), and the handler forwards straight to the plugin via
// ForwardToPlugin -- it must NOT re-Dispatch the command string (that would re-match
// this builtin and recurse). The ze-show:ospf-* methods are CENTRAL-namespace RPCs
// registered here in Go; ze-ospf-cmd.yang ships the command-tree nodes that bind them.

package ospf

import (
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
	"codeberg.org/thomas-mangin/ze/internal/core/textbuf"
)

// Plugin command names -- shared by RPCRegistration.PluginCommand and forwardToOSPF so
// the two cannot diverge. They match the CommandDecl names runOSPFEngine registers.
const (
	cmdShowProcess       = "show ospf"
	cmdShowNeighbor      = "show ospf neighbor"
	cmdShowInterface     = "show ospf interface"
	cmdShowDatabase      = "show ospf database"
	cmdShowRoute         = "show ospf route"
	cmdShowBorderRouters = "show ospf border-routers"
	cmdShowSPF           = "show ospf spf"
	cmdClearProcess      = "clear ospf process"
	cmdClearNeighbor     = "clear ospf neighbor"
	cmdClearCounters     = "clear ospf counters"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-show:ospf", Handler: forwardShowOSPFProcess, PluginCommand: cmdShowProcess},
		pluginserver.RPCRegistration{WireMethod: "ze-show:ospf-neighbor", Handler: forwardShowOSPFNeighbor, PluginCommand: cmdShowNeighbor},
		pluginserver.RPCRegistration{WireMethod: "ze-show:ospf-interface", Handler: forwardShowOSPFInterface, PluginCommand: cmdShowInterface},
		pluginserver.RPCRegistration{WireMethod: "ze-show:ospf-database", Handler: forwardShowOSPFDatabase, PluginCommand: cmdShowDatabase},
		pluginserver.RPCRegistration{WireMethod: "ze-show:ospf-route", Handler: forwardShowOSPFRoute, PluginCommand: cmdShowRoute},
		pluginserver.RPCRegistration{WireMethod: "ze-show:ospf-border-routers", Handler: forwardShowOSPFBorderRouters, PluginCommand: cmdShowBorderRouters},
		pluginserver.RPCRegistration{WireMethod: "ze-show:ospf-spf", Handler: forwardShowOSPFSPF, PluginCommand: cmdShowSPF},
		pluginserver.RPCRegistration{WireMethod: "ze-show:ospf-database-router", Handler: dbSubviewForwarder("show ospf database router"), PluginCommand: "show ospf database router"},
		pluginserver.RPCRegistration{WireMethod: "ze-show:ospf-database-network", Handler: dbSubviewForwarder("show ospf database network"), PluginCommand: "show ospf database network"},
		pluginserver.RPCRegistration{WireMethod: "ze-show:ospf-database-summary", Handler: dbSubviewForwarder("show ospf database summary"), PluginCommand: "show ospf database summary"},
		pluginserver.RPCRegistration{WireMethod: "ze-show:ospf-database-asbr-summary", Handler: dbSubviewForwarder("show ospf database asbr-summary"), PluginCommand: "show ospf database asbr-summary"},
		pluginserver.RPCRegistration{WireMethod: "ze-show:ospf-database-external", Handler: dbSubviewForwarder("show ospf database external"), PluginCommand: "show ospf database external"},
		pluginserver.RPCRegistration{WireMethod: "ze-show:ospf-database-nssa-external", Handler: dbSubviewForwarder("show ospf database nssa-external"), PluginCommand: "show ospf database nssa-external"},
		pluginserver.RPCRegistration{WireMethod: "ze-clear:ospf-process", Handler: forwardClearOSPFProcess, PluginCommand: cmdClearProcess},
		pluginserver.RPCRegistration{WireMethod: "ze-clear:ospf-neighbor", Handler: forwardClearOSPFNeighbor, PluginCommand: cmdClearNeighbor},
		pluginserver.RPCRegistration{WireMethod: "ze-clear:ospf-counters", Handler: forwardClearOSPFCounters, PluginCommand: cmdClearCounters},
	)
}

func forwardShowOSPFProcess(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return forwardToOSPF(ctx, cmdShowProcess, args)
}

func forwardShowOSPFNeighbor(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return forwardToOSPF(ctx, cmdShowNeighbor, args)
}

func forwardShowOSPFInterface(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return forwardToOSPF(ctx, cmdShowInterface, args)
}

func forwardShowOSPFDatabase(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return forwardToOSPF(ctx, cmdShowDatabase, args)
}

func forwardShowOSPFRoute(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return forwardToOSPF(ctx, cmdShowRoute, args)
}

func forwardShowOSPFBorderRouters(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return forwardToOSPF(ctx, cmdShowBorderRouters, args)
}

func forwardShowOSPFSPF(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return forwardToOSPF(ctx, cmdShowSPF, args)
}

func forwardClearOSPFProcess(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return forwardToOSPF(ctx, cmdClearProcess, args)
}

func forwardClearOSPFNeighbor(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return forwardToOSPF(ctx, cmdClearNeighbor, args)
}

func forwardClearOSPFCounters(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return forwardToOSPF(ctx, cmdClearCounters, args)
}

// dbSubviewForwarder builds a handler that proxies one `show ospf database <type>`
// subview to the engine. A closure avoids six near-identical named functions.
func dbSubviewForwarder(command string) pluginserver.Handler {
	return func(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
		return forwardToOSPF(ctx, command, args)
	}
}

// forwardToOSPF routes a fixed plugin command to the OSPF engine. The proxied commands
// accept no arguments (the noun is baked into the fixed command string by the grammar),
// so any extra args are rejected (the LDP/IS-IS proxy contract).
func forwardToOSPF(ctx *pluginserver.CommandContext, command string, args []string) (*plugin.Response, error) {
	if len(args) > 0 {
		var tb textbuf.Buffer
		return &plugin.Response{Status: plugin.StatusError, Error: tb.Str("unexpected argument; ").Str(command).Str(" takes none").String()}, nil
	}
	d := ctx.Dispatcher()
	if d == nil {
		return &plugin.Response{Status: plugin.StatusError, Error: "dispatcher unavailable"}, nil
	}
	return d.ForwardToPlugin(ctx, command, args, ctx.PeerSelector())
}
