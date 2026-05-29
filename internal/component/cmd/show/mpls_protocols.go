// Design: plan/spec-mpls-2-ldp.md, plan/spec-mpls-3-rsvp-te.md --
//
//	`show rsvp-te ...` / `show ldp ...` surfaced under the top-level show grammar.
//
// Related: mpls_forwarding.go -- sibling show handler for the kernel MPLS table.
// Related: ../../rsvpte/register.go, ../../ldp/register.go -- the proxied plugin commands.
//
// The RSVP-TE and LDP introspection data lives in their plugin processes,
// reachable via the commands the components register ("rsvp-te show-session",
// "ldp show-neighbor", ...). These show handlers proxy to those commands through
// the dispatcher and relay the response, giving operators a single `show` entry
// point. If the plugin is not running the dispatcher reports the command as
// unknown, surfaced here as an operational error rather than a crash.
package show

import (
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-show:rsvp-te-lsp", Handler: proxyShowToPlugin("rsvp-te show-session")},
		pluginserver.RPCRegistration{WireMethod: "ze-show:rsvp-te-interface", Handler: proxyShowToPlugin("rsvp-te show-interface")},
		pluginserver.RPCRegistration{WireMethod: "ze-show:rsvp-te-tunnel", Handler: proxyShowToPlugin("rsvp-te show-tunnel")},
		pluginserver.RPCRegistration{WireMethod: "ze-show:ldp-neighbor", Handler: proxyShowToPlugin("ldp show-neighbor")},
		pluginserver.RPCRegistration{WireMethod: "ze-show:ldp-binding", Handler: proxyShowToPlugin("ldp show-binding")},
	)
}

// proxyShowToPlugin returns a show handler that dispatches a fixed plugin
// command and relays its response unchanged. The proxied plugin commands accept
// no arguments, so any extra args are rejected.
func proxyShowToPlugin(command string) func(*pluginserver.CommandContext, []string) (*plugin.Response, error) {
	return func(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
		if len(args) > 0 {
			return &plugin.Response{Status: plugin.StatusError, Error: "unexpected argument; show " + command + " takes none"}, nil
		}
		d := ctx.Dispatcher()
		if d == nil {
			return &plugin.Response{Status: plugin.StatusError, Error: "dispatcher unavailable"}, nil
		}
		return d.Dispatch(ctx, command)
	}
}
