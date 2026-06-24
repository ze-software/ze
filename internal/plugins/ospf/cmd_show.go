// Design: plan/learned/967-ospf-13-cli-diag-interop.md -- `show ip ospf ...` under the
// top-level show grammar.
// Related: register.go -- runOSPFEngine registers the matching CommandDecl set and the
// OnExecuteCommand render dispatch.
// Related: yang/ze-ospf-cmd.yang -- the owner command tree binding these wire methods.
//
// The OSPF introspection data lives in the engine plugin process, reachable via the
// commands the component registers ("show ip ospf neighbor", ...). These builtin RPCs
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
	cmdShowProcess       = "show ip ospf"
	cmdShowNeighbor      = "show ip ospf neighbor"
	cmdShowInterface     = "show ip ospf interface"
	cmdShowDatabase      = "show ip ospf database"
	cmdShowRoute         = "show ip ospf route"
	cmdShowBorderRouters = "show ip ospf border-routers"
	cmdShowSPF           = "show ip ospf spf"
	cmdClearProcess      = "clear ip ospf process"
	cmdClearNeighbor     = "clear ip ospf neighbor"
	cmdClearCounters     = "clear ip ospf counters"
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
		pluginserver.RPCRegistration{WireMethod: "ze-show:ospf-database-router", Handler: dbSubviewForwarder("show ip ospf database router"), PluginCommand: "show ip ospf database router"},
		pluginserver.RPCRegistration{WireMethod: "ze-show:ospf-database-network", Handler: dbSubviewForwarder("show ip ospf database network"), PluginCommand: "show ip ospf database network"},
		pluginserver.RPCRegistration{WireMethod: "ze-show:ospf-database-summary", Handler: dbSubviewForwarder("show ip ospf database summary"), PluginCommand: "show ip ospf database summary"},
		pluginserver.RPCRegistration{WireMethod: "ze-show:ospf-database-asbr-summary", Handler: dbSubviewForwarder("show ip ospf database asbr-summary"), PluginCommand: "show ip ospf database asbr-summary"},
		pluginserver.RPCRegistration{WireMethod: "ze-show:ospf-database-external", Handler: dbSubviewForwarder("show ip ospf database external"), PluginCommand: "show ip ospf database external"},
		pluginserver.RPCRegistration{WireMethod: "ze-show:ospf-database-nssa-external", Handler: dbSubviewForwarder("show ip ospf database nssa-external"), PluginCommand: "show ip ospf database nssa-external"},
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

// dbSubviewForwarder builds a handler that proxies one `show ip ospf database <type>`
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
