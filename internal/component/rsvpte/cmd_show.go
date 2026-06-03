// Design: plan/spec-mpls-3-rsvp-te.md -- `show rsvp-te ...` surfaced under the
// top-level show grammar.
//
// The RSVP-TE introspection data lives in the plugin process, reachable via the
// commands the component registers ("show rsvp-te session", ...). These show
// handlers proxy to those commands through the dispatcher and relay the
// response, giving operators a single `show` entry point. Owned by the rsvp-te
// component so that removing it removes the `show rsvp-te ...` command, its
// schema, and these handlers together. See ai/rules/plugin-self-containment.md.

package rsvpte

import (
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-show:rsvp-te-lsp", Handler: proxyShowToPlugin("show rsvp-te session")},
		pluginserver.RPCRegistration{WireMethod: "ze-show:rsvp-te-interface", Handler: proxyShowToPlugin("show rsvp-te interface")},
		pluginserver.RPCRegistration{WireMethod: "ze-show:rsvp-te-tunnel", Handler: proxyShowToPlugin("show rsvp-te tunnel")},
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
