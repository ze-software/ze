// Design: docs/architecture/isis/isis-13-cli-diag-interop.md -- `show isis ...` / `clear isis
// ...` surfaced under the top-level show/clear grammar.
// Related: register.go -- runISISEngine registers the matching CommandDecl set
// Related: show.go -- the engine-side render/clear OnExecuteCommand dispatches to
// Related: yang/ze-isis-cmd.yang -- the owner command tree binding these wire methods
//
// The IS-IS introspection data lives in the engine plugin process, reachable via
// the commands the component registers ("show isis neighbor", ...). These builtin
// RPCs are plugin proxies (the LDP model, internal/plugins/ldp/cmd_show.go):
// each declares the plugin command it fronts via PluginCommand, which lets the
// IS-IS engine register the same command name (instead of being rejected as a
// builtin conflict) and marks the builtin so it does NOT claim the command name
// in the registry. The handler then forwards straight to the plugin process
// through ForwardToPlugin -- it must NOT re-Dispatch the command string, which
// would re-match this same builtin and recurse until the stack overflows. Both
// the show methods (ze-show:isis-*) and the clear actions (ze-clear:isis-*) are
// CENTRAL-namespace RPCs registered here in Go (no per-component ze-isis-api
// module); the owner command YANG (yang/ze-isis-cmd.yang) ships the command-tree
// nodes that bind them. Owned by the isis component so removing it removes the
// `show isis ...` / `clear isis ...` commands, their schema, and these handlers
// together (ai/rules/plugins.md).

package isis

import (
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// Plugin command names -- used in both RPCRegistration.PluginCommand and
// forwardToISIS so the two cannot diverge. They match the CommandDecl names the
// IS-IS engine registers in runISISEngine's sdk.Registration.
const (
	cmdShowNeighbor       = "show isis neighbor"
	cmdShowDatabase       = "show isis database"
	cmdShowDatabaseDetail = "show isis database detail"
	cmdShowRoute          = "show isis route"
	cmdShowRouteIPv6      = "show isis route ipv6"
	cmdShowInterface      = "show isis interface"
	cmdShowHostname       = "show isis hostname"
	cmdShowSPFLog         = "show isis spf-log"
	cmdClearAdjacency     = "clear isis adjacency"
	cmdClearCounters      = "clear isis counters"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-show:isis-neighbor", Handler: forwardShowNeighbor, PluginCommand: cmdShowNeighbor},
		pluginserver.RPCRegistration{WireMethod: "ze-show:isis-database", Handler: forwardShowDatabase, PluginCommand: cmdShowDatabase},
		pluginserver.RPCRegistration{WireMethod: "ze-show:isis-database-detail", Handler: forwardShowDatabaseDetail, PluginCommand: cmdShowDatabaseDetail},
		pluginserver.RPCRegistration{WireMethod: "ze-show:isis-route", Handler: forwardShowRoute, PluginCommand: cmdShowRoute},
		pluginserver.RPCRegistration{WireMethod: "ze-show:isis-route-ipv6", Handler: forwardShowRouteIPv6, PluginCommand: cmdShowRouteIPv6},
		pluginserver.RPCRegistration{WireMethod: "ze-show:isis-interface", Handler: forwardShowInterface, PluginCommand: cmdShowInterface},
		pluginserver.RPCRegistration{WireMethod: "ze-show:isis-hostname", Handler: forwardShowHostname, PluginCommand: cmdShowHostname},
		pluginserver.RPCRegistration{WireMethod: "ze-show:isis-spf-log", Handler: forwardShowSPFLog, PluginCommand: cmdShowSPFLog},
		pluginserver.RPCRegistration{WireMethod: "ze-clear:isis-adjacency", Handler: forwardClearAdjacency, PluginCommand: cmdClearAdjacency},
		pluginserver.RPCRegistration{WireMethod: "ze-clear:isis-counters", Handler: forwardClearCounters, PluginCommand: cmdClearCounters},
	)
}

func forwardShowNeighbor(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return forwardToISIS(ctx, cmdShowNeighbor, args)
}

func forwardShowDatabase(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return forwardToISIS(ctx, cmdShowDatabase, args)
}

func forwardShowDatabaseDetail(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return forwardToISIS(ctx, cmdShowDatabaseDetail, args)
}

func forwardShowRoute(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return forwardToISIS(ctx, cmdShowRoute, args)
}

func forwardShowRouteIPv6(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return forwardToISIS(ctx, cmdShowRouteIPv6, args)
}

func forwardShowInterface(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return forwardToISIS(ctx, cmdShowInterface, args)
}

func forwardShowHostname(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return forwardToISIS(ctx, cmdShowHostname, args)
}

func forwardShowSPFLog(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return forwardToISIS(ctx, cmdShowSPFLog, args)
}

func forwardClearAdjacency(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return forwardToISIS(ctx, cmdClearAdjacency, args)
}

func forwardClearCounters(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return forwardToISIS(ctx, cmdClearCounters, args)
}

// forwardToISIS routes a fixed plugin command to the IS-IS engine. The proxied
// commands accept no arguments (the `detail` and noun keywords are baked into
// the fixed command string by the grammar), so any extra args are rejected
// (matches the LDP proxy contract, spec-isis-13 AC-7 proxy-arg test).
func forwardToISIS(ctx *pluginserver.CommandContext, command string, args []string) (*plugin.Response, error) {
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
