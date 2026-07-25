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
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// Plugin command names -- shared by RPCRegistration.PluginCommand and forwardToOSPF so
// the two cannot diverge. They match the CommandDecl names runOSPFEngine registers.
const (
	cmdShowProcess             = "show ospf"
	cmdShowIPv6                = "show ospf ipv6"
	cmdShowInstance            = "show ospf instance"
	cmdShowNeighbor            = "show ospf neighbor"
	cmdShowInterface           = "show ospf interface"
	cmdShowIPv6Interface       = "show ospf ipv6 interface"
	cmdShowDatabase            = "show ospf database"
	cmdShowDatabaseRI          = "show ospf database router-information"
	cmdShowTEDatabase          = "show ospf te-database"
	cmdShowRoute               = "show ospf route"
	cmdShowRouteFastReroute    = "show ospf route fast-reroute"
	cmdShowVirtualLinks        = "show ospf virtual-links"
	cmdShowBorderRouters       = "show ospf border-routers"
	cmdShowSPF                 = "show ospf spf"
	cmdShowLDPSync             = "show ospf ldp-sync"
	cmdShowGracefulRestart     = "show ospf graceful-restart"
	cmdShowIPv6GracefulRestart = "show ospf ipv6 graceful-restart"
	cmdShowSegmentRouting      = "show ospf segment-routing"
	cmdShowIPv6SegmentRouting  = "show ospf ipv6 segment-routing"
	cmdClearProcess            = "clear ospf process"
	cmdClearNeighbor           = "clear ospf neighbor"
	cmdClearCounters           = "clear ospf counters"
	cmdGRPrepare               = "request ospf graceful-restart"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-show:ospf", Handler: forwardShowOSPFProcess, PluginCommand: cmdShowProcess},
		pluginserver.RPCRegistration{WireMethod: "ze-show:ospf-ipv6", Handler: forwardShowOSPFIPv6, PluginCommand: cmdShowIPv6},
		pluginserver.RPCRegistration{WireMethod: "ze-show:ospf-instance", Handler: forwardShowOSPFInstance, PluginCommand: cmdShowInstance},
		pluginserver.RPCRegistration{WireMethod: "ze-show:ospf-neighbor", Handler: forwardShowOSPFNeighbor, PluginCommand: cmdShowNeighbor},
		pluginserver.RPCRegistration{WireMethod: "ze-show:ospf-interface", Handler: forwardShowOSPFInterface, PluginCommand: cmdShowInterface},
		pluginserver.RPCRegistration{WireMethod: "ze-show:ospf-ipv6-interface", Handler: forwardShowOSPFIPv6Interface, PluginCommand: cmdShowIPv6Interface},
		pluginserver.RPCRegistration{WireMethod: "ze-show:ospf-database", Handler: forwardShowOSPFDatabase, PluginCommand: cmdShowDatabase},
		pluginserver.RPCRegistration{WireMethod: "ze-show:ospf-route", Handler: forwardShowOSPFRoute, PluginCommand: cmdShowRoute},
		pluginserver.RPCRegistration{WireMethod: "ze-show:ospf-route-fast-reroute", Handler: forwardShowOSPFRouteFastReroute, PluginCommand: cmdShowRouteFastReroute},
		pluginserver.RPCRegistration{WireMethod: "ze-show:ospf-virtual-links", Handler: forwardShowOSPFVirtualLinks, PluginCommand: cmdShowVirtualLinks},
		pluginserver.RPCRegistration{WireMethod: "ze-show:ospf-border-routers", Handler: forwardShowOSPFBorderRouters, PluginCommand: cmdShowBorderRouters},
		pluginserver.RPCRegistration{WireMethod: "ze-show:ospf-spf", Handler: forwardShowOSPFSPF, PluginCommand: cmdShowSPF},
		pluginserver.RPCRegistration{WireMethod: "ze-show:ospf-ldp-sync", Handler: forwardShowOSPFLDPSync, PluginCommand: cmdShowLDPSync},
		pluginserver.RPCRegistration{WireMethod: "ze-show:ospf-graceful-restart", Handler: forwardShowOSPFGracefulRestart, PluginCommand: cmdShowGracefulRestart},
		pluginserver.RPCRegistration{WireMethod: "ze-show:ospf-ipv6-graceful-restart", Handler: forwardShowOSPFIPv6GracefulRestart, PluginCommand: cmdShowIPv6GracefulRestart},
		pluginserver.RPCRegistration{WireMethod: "ze-show:ospf-segment-routing", Handler: forwardShowOSPFSegmentRouting, PluginCommand: cmdShowSegmentRouting},
		pluginserver.RPCRegistration{WireMethod: "ze-show:ospf-ipv6-segment-routing", Handler: forwardShowOSPFIPv6SegmentRouting, PluginCommand: cmdShowIPv6SegmentRouting},
		pluginserver.RPCRegistration{WireMethod: "ze-show:ospf-database-router", Handler: dbSubviewForwarder("show ospf database router"), PluginCommand: "show ospf database router"},
		pluginserver.RPCRegistration{WireMethod: "ze-show:ospf-database-network", Handler: dbSubviewForwarder("show ospf database network"), PluginCommand: "show ospf database network"},
		pluginserver.RPCRegistration{WireMethod: "ze-show:ospf-database-summary", Handler: dbSubviewForwarder("show ospf database summary"), PluginCommand: "show ospf database summary"},
		pluginserver.RPCRegistration{WireMethod: "ze-show:ospf-database-asbr-summary", Handler: dbSubviewForwarder("show ospf database asbr-summary"), PluginCommand: "show ospf database asbr-summary"},
		pluginserver.RPCRegistration{WireMethod: "ze-show:ospf-database-external", Handler: dbSubviewForwarder("show ospf database external"), PluginCommand: "show ospf database external"},
		pluginserver.RPCRegistration{WireMethod: "ze-show:ospf-database-nssa-external", Handler: dbSubviewForwarder("show ospf database nssa-external"), PluginCommand: "show ospf database nssa-external"},
		pluginserver.RPCRegistration{WireMethod: "ze-show:ospf-database-opaque-link", Handler: dbSubviewForwarder("show ospf database opaque-link"), PluginCommand: "show ospf database opaque-link"},
		pluginserver.RPCRegistration{WireMethod: "ze-show:ospf-database-opaque-area", Handler: dbSubviewForwarder("show ospf database opaque-area"), PluginCommand: "show ospf database opaque-area"},
		pluginserver.RPCRegistration{WireMethod: "ze-show:ospf-database-opaque-as", Handler: dbSubviewForwarder("show ospf database opaque-as"), PluginCommand: "show ospf database opaque-as"},
		pluginserver.RPCRegistration{WireMethod: "ze-show:ospf-database-router-information", Handler: dbSubviewForwarder(cmdShowDatabaseRI), PluginCommand: cmdShowDatabaseRI},
		pluginserver.RPCRegistration{WireMethod: "ze-show:ospf-te-database", Handler: dbSubviewForwarder(cmdShowTEDatabase), PluginCommand: cmdShowTEDatabase},
		pluginserver.RPCRegistration{WireMethod: "ze-clear:ospf-process", Handler: forwardClearOSPFProcess, PluginCommand: cmdClearProcess},
		pluginserver.RPCRegistration{WireMethod: "ze-clear:ospf-neighbor", Handler: forwardClearOSPFNeighbor, PluginCommand: cmdClearNeighbor},
		pluginserver.RPCRegistration{WireMethod: "ze-clear:ospf-counters", Handler: forwardClearOSPFCounters, PluginCommand: cmdClearCounters},
		pluginserver.RPCRegistration{WireMethod: "ze-ospf:graceful-restart-prepare", Handler: forwardOSPFGRPrepare, PluginCommand: cmdGRPrepare},

		// spec-ospf-ext-14 IPv4 deep-introspection views.
		pluginserver.RPCRegistration{WireMethod: "ze-show:ospf-database-opaque-area-detail", Handler: dbSubviewForwarder("show ospf database opaque-area detail"), PluginCommand: "show ospf database opaque-area detail"},
		pluginserver.RPCRegistration{WireMethod: "ze-show:ospf-database-opaque-as-detail", Handler: dbSubviewForwarder("show ospf database opaque-as detail"), PluginCommand: "show ospf database opaque-as detail"},
		pluginserver.RPCRegistration{WireMethod: "ze-show:ospf-database-opaque-link-detail", Handler: dbSubviewForwarder("show ospf database opaque-link detail"), PluginCommand: "show ospf database opaque-link detail"},
		pluginserver.RPCRegistration{WireMethod: "ze-show:ospf-spf-detail", Handler: dbSubviewForwarder("show ospf spf detail"), PluginCommand: "show ospf spf detail"},
		pluginserver.RPCRegistration{WireMethod: "ze-show:ospf-neighbor-detail", Handler: dbSubviewForwarder("show ospf neighbor detail"), PluginCommand: "show ospf neighbor detail"},
		pluginserver.RPCRegistration{WireMethod: "ze-show:ospf-interface-detail", Handler: dbSubviewForwarder("show ospf interface detail"), PluginCommand: "show ospf interface detail"},

		// spec-ospf-ext-14 IPv6 (OSPFv3) deep-introspection views. Distinct ze-show:ospfv3-*
		// wire methods so the v4 and v6 surfaces cannot collide (AC-26, R-9).
		pluginserver.RPCRegistration{WireMethod: "ze-show:ospfv3-database", Handler: dbSubviewForwarder("show ospf ipv6 database"), PluginCommand: "show ospf ipv6 database"},
		pluginserver.RPCRegistration{WireMethod: "ze-show:ospfv3-database-detail", Handler: dbSubviewForwarder("show ospf ipv6 database detail"), PluginCommand: "show ospf ipv6 database detail"},
		pluginserver.RPCRegistration{WireMethod: "ze-show:ospfv3-database-router-detail", Handler: dbSubviewForwarder("show ospf ipv6 database router detail"), PluginCommand: "show ospf ipv6 database router detail"},
		pluginserver.RPCRegistration{WireMethod: "ze-show:ospfv3-database-scope-link", Handler: dbSubviewForwarder("show ospf ipv6 database scope link"), PluginCommand: "show ospf ipv6 database scope link"},
		pluginserver.RPCRegistration{WireMethod: "ze-show:ospfv3-database-scope-area", Handler: dbSubviewForwarder("show ospf ipv6 database scope area"), PluginCommand: "show ospf ipv6 database scope area"},
		pluginserver.RPCRegistration{WireMethod: "ze-show:ospfv3-database-scope-as", Handler: dbSubviewForwarder("show ospf ipv6 database scope as"), PluginCommand: "show ospf ipv6 database scope as"},
		pluginserver.RPCRegistration{WireMethod: "ze-show:ospfv3-database-router-information", Handler: dbSubviewForwarder("show ospf ipv6 database router-information"), PluginCommand: "show ospf ipv6 database router-information"},
		pluginserver.RPCRegistration{WireMethod: "ze-show:ospfv3-database-extended", Handler: dbSubviewForwarder("show ospf ipv6 database extended"), PluginCommand: "show ospf ipv6 database extended"},
		pluginserver.RPCRegistration{WireMethod: "ze-show:ospfv3-database-segment-routing", Handler: dbSubviewForwarder("show ospf ipv6 database segment-routing"), PluginCommand: "show ospf ipv6 database segment-routing"},
		pluginserver.RPCRegistration{WireMethod: "ze-show:ospfv3-instance", Handler: dbSubviewForwarder("show ospf ipv6 instance"), PluginCommand: "show ospf ipv6 instance"},
		pluginserver.RPCRegistration{WireMethod: "ze-show:ospfv3-neighbor", Handler: dbSubviewForwarder("show ospf ipv6 neighbor"), PluginCommand: "show ospf ipv6 neighbor"},
		pluginserver.RPCRegistration{WireMethod: "ze-show:ospfv3-neighbor-detail", Handler: dbSubviewForwarder("show ospf ipv6 neighbor detail"), PluginCommand: "show ospf ipv6 neighbor detail"},
		pluginserver.RPCRegistration{WireMethod: "ze-show:ospfv3-interface-detail", Handler: dbSubviewForwarder("show ospf ipv6 interface detail"), PluginCommand: "show ospf ipv6 interface detail"},
		pluginserver.RPCRegistration{WireMethod: "ze-show:ospfv3-spf", Handler: dbSubviewForwarder("show ospf ipv6 spf"), PluginCommand: "show ospf ipv6 spf"},
		pluginserver.RPCRegistration{WireMethod: "ze-show:ospfv3-spf-detail", Handler: dbSubviewForwarder("show ospf ipv6 spf detail"), PluginCommand: "show ospf ipv6 spf detail"},

		// spec-ospf-ext-14 guarded LSA injection (both families). The inject proxies pass the
		// trailing keyword-value tokens through as args; the read-only authz `deny debug`
		// blocks them before dispatch (AC-16) and the engine debug-enablement is the 2nd gate.
		pluginserver.RPCRegistration{WireMethod: "ze-debug:ospf-inject", Handler: forwardOSPFInjectV4, PluginCommand: "debug ip ospf inject opaque"},
		pluginserver.RPCRegistration{WireMethod: "ze-debug:ospfv3-inject", Handler: forwardOSPFInjectV6, PluginCommand: "debug ipv6 ospf inject lsa"},
		pluginserver.RPCRegistration{WireMethod: "ze-debug:ospf-inject-enable", Handler: dbSubviewForwarder("debug ospf inject enable"), PluginCommand: "debug ospf inject enable"},
		pluginserver.RPCRegistration{WireMethod: "ze-debug:ospf-inject-disable", Handler: dbSubviewForwarder("debug ospf inject disable"), PluginCommand: "debug ospf inject disable"},
	)
}

// forwardOSPFInjectV4 proxies `debug ip ospf inject opaque ...` to the OSPF engine, passing
// the trailing scope/id/type/hex tokens through as args (the inject grammar is variable).
func forwardOSPFInjectV4(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return forwardToOSPFArgs(ctx, "debug ip ospf inject opaque", args)
}

// forwardOSPFInjectV6 proxies `debug ipv6 ospf inject lsa ...` to the OSPF engine.
func forwardOSPFInjectV6(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return forwardToOSPFArgs(ctx, "debug ipv6 ospf inject lsa", args)
}

// forwardToOSPFArgs proxies a command AND its trailing arguments to the OSPF engine (unlike
// forwardToOSPF, which rejects extra args for the fixed-noun show/clear commands).
func forwardToOSPFArgs(ctx *pluginserver.CommandContext, command string, args []string) (*plugin.Response, error) {
	d := ctx.Dispatcher()
	if d == nil {
		return &plugin.Response{Status: plugin.StatusError, Error: "dispatcher unavailable"}, nil
	}
	return d.ForwardToPlugin(ctx, command, args, ctx.PeerSelector())
}

func forwardShowOSPFProcess(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return forwardToOSPF(ctx, cmdShowProcess, args)
}

func forwardShowOSPFIPv6(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return forwardToOSPF(ctx, cmdShowIPv6, args)
}

func forwardShowOSPFInstance(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return forwardToOSPF(ctx, cmdShowInstance, args)
}

func forwardShowOSPFNeighbor(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return forwardToOSPF(ctx, cmdShowNeighbor, args)
}

func forwardShowOSPFInterface(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return forwardToOSPF(ctx, cmdShowInterface, args)
}

func forwardShowOSPFIPv6Interface(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return forwardToOSPF(ctx, cmdShowIPv6Interface, args)
}

func forwardShowOSPFDatabase(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return forwardToOSPF(ctx, cmdShowDatabase, args)
}

func forwardShowOSPFRoute(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return forwardToOSPF(ctx, cmdShowRoute, args)
}

func forwardShowOSPFRouteFastReroute(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return forwardToOSPF(ctx, cmdShowRouteFastReroute, args)
}

func forwardShowOSPFVirtualLinks(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return forwardToOSPF(ctx, cmdShowVirtualLinks, args)
}

func forwardShowOSPFBorderRouters(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return forwardToOSPF(ctx, cmdShowBorderRouters, args)
}

func forwardShowOSPFSPF(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return forwardToOSPF(ctx, cmdShowSPF, args)
}

func forwardShowOSPFLDPSync(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return forwardToOSPF(ctx, cmdShowLDPSync, args)
}

func forwardShowOSPFGracefulRestart(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return forwardToOSPF(ctx, cmdShowGracefulRestart, args)
}

func forwardShowOSPFIPv6GracefulRestart(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return forwardToOSPF(ctx, cmdShowIPv6GracefulRestart, args)
}

func forwardShowOSPFSegmentRouting(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return forwardToOSPF(ctx, cmdShowSegmentRouting, args)
}

func forwardShowOSPFIPv6SegmentRouting(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return forwardToOSPF(ctx, cmdShowIPv6SegmentRouting, args)
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

// forwardOSPFGRPrepare proxies the operator `request ospf graceful-restart` action to the OSPF
// engine, which runs prepareRestart against its live state (RFC 3623 sec 2.1). Same forwarding
// contract as the clear commands: the noun is fixed in the grammar, so no arguments are taken.
func forwardOSPFGRPrepare(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return forwardToOSPF(ctx, cmdGRPrepare, args)
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
