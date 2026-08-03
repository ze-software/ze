// Design: plan/learned/921-mpls-rsvp-te.md -- `show rsvp-te ...` surfaced under the
// top-level show grammar.
// Related: show_data.go -- the show data builders these RPC proxies forward to
//
// The RSVP-TE introspection data lives in the plugin engine, reachable via the
// commands the component registers ("show rsvp-te session", ...). These builtin
// RPCs are plugin proxies: each declares the plugin command it fronts via
// PluginCommand, which lets the RSVP-TE engine register the same command name
// (instead of being rejected as a builtin conflict) and marks the builtin so it
// does NOT claim the command name in the registry. The handler then forwards
// straight to the plugin process through ForwardToPlugin -- it must NOT
// re-Dispatch the command string, which would re-match this same builtin and
// recurse until the stack overflows. Owned by the rsvp-te component so that
// removing it removes the `show rsvp-te ...` command, its schema, and these
// handlers together. See ai/rules/plugins.md and
// internal/component/bgp/plugins/cmd/rib/rib.go for the canonical proxy.

package rsvpte

import (
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// Plugin command names -- used in both RPCRegistration.PluginCommand and
// ForwardToPlugin so the two cannot diverge. They match the CommandDecl names
// the RSVP-TE engine registers in runRSVPTEEngine.
const (
	cmdShowSession     = "show rsvp-te session"
	cmdShowInterface   = "show rsvp-te interface"
	cmdShowTunnel      = "show rsvp-te tunnel"
	cmdShowFastReroute = "show rsvp-te fast-reroute"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-show:rsvp-te-lsp", Handler: forwardShowSession, PluginCommand: cmdShowSession},
		pluginserver.RPCRegistration{WireMethod: "ze-show:rsvp-te-interface", Handler: forwardShowInterface, PluginCommand: cmdShowInterface},
		pluginserver.RPCRegistration{WireMethod: "ze-show:rsvp-te-tunnel", Handler: forwardShowTunnel, PluginCommand: cmdShowTunnel},
		pluginserver.RPCRegistration{WireMethod: "ze-show:rsvp-te-fast-reroute", Handler: forwardShowFastReroute, PluginCommand: cmdShowFastReroute},
	)
}

func forwardShowSession(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return forwardToRSVPTE(ctx, cmdShowSession, args)
}

func forwardShowInterface(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return forwardToRSVPTE(ctx, cmdShowInterface, args)
}

func forwardShowTunnel(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return forwardToRSVPTE(ctx, cmdShowTunnel, args)
}

func forwardShowFastReroute(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return forwardToRSVPTE(ctx, cmdShowFastReroute, args)
}

// forwardToRSVPTE routes a fixed plugin command to the RSVP-TE engine. The
// proxied commands accept no arguments, so any extra args are rejected.
func forwardToRSVPTE(ctx *pluginserver.CommandContext, command string, args []string) (*plugin.Response, error) {
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
