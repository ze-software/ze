// Design: docs/architecture/ldp/mpls-ldp.md -- `show ldp ...` surfaced under the top-level
// show grammar.
//
// The LDP introspection data lives in the plugin engine, reachable via the
// commands the component registers ("show ldp neighbor", ...). These builtin
// RPCs are plugin proxies: each declares the plugin command it fronts via
// PluginCommand, which lets the LDP engine register the same command name
// (instead of being rejected as a builtin conflict) and marks the builtin so it
// does NOT claim the command name in the registry. The handler then forwards
// straight to the plugin process through ForwardToPlugin -- it must NOT
// re-Dispatch the command string, which would re-match this same builtin and
// recurse until the stack overflows. Owned by the ldp component so that removing
// it removes the `show ldp ...` command, its schema, and these handlers
// together. See ai/rules/plugins.md and
// internal/component/bgp/plugins/cmd/rib/rib.go for the canonical proxy.

package ldp

import (
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// Plugin command names -- used in both RPCRegistration.PluginCommand and
// ForwardToPlugin so the two cannot diverge. They match the CommandDecl names
// the LDP engine registers in runLDPEngine.
const (
	cmdShowNeighbor = "show ldp neighbor"
	cmdShowBinding  = "show ldp binding"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-show:ldp-neighbor", Handler: forwardShowNeighbor, PluginCommand: cmdShowNeighbor},
		pluginserver.RPCRegistration{WireMethod: "ze-show:ldp-binding", Handler: forwardShowBinding, PluginCommand: cmdShowBinding},
	)
}

func forwardShowNeighbor(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return forwardToLDP(ctx, cmdShowNeighbor, args)
}

func forwardShowBinding(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return forwardToLDP(ctx, cmdShowBinding, args)
}

// forwardToLDP routes a fixed plugin command to the LDP engine. The proxied
// commands accept no arguments, so any extra args are rejected.
func forwardToLDP(ctx *pluginserver.CommandContext, command string, args []string) (*plugin.Response, error) {
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
