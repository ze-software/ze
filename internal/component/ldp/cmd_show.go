// Design: plan/spec-mpls-2-ldp.md -- `show ldp ...` surfaced under the top-level
// show grammar.
//
// The LDP introspection data lives in the plugin process, reachable via the
// commands the component registers ("show ldp neighbor", ...). These show
// handlers proxy to those commands through the dispatcher and relay the
// response, giving operators a single `show` entry point. Owned by the ldp
// component so that removing it removes the `show ldp ...` command, its schema,
// and these handlers together. See ai/rules/plugin-self-containment.md.

package ldp

import (
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-show:ldp-neighbor", Handler: proxyShowToPlugin("show ldp neighbor")},
		pluginserver.RPCRegistration{WireMethod: "ze-show:ldp-binding", Handler: proxyShowToPlugin("show ldp binding")},
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
