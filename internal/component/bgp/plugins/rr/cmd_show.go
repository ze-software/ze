// Design: docs/architecture/api/commands.md — show rr proxy handlers.
// Owned by the bgp-rr plugin so that removing the route-reflector surface
// removes the `show rr ...` command, its schema, and these handlers together.
// See ai/rules/plugins.md.

package rr

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

const (
	cmdShowRRStatus = "show rr status"
	cmdShowRRPeers  = "show rr peers"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{
			WireMethod:    "ze-bgp:show-rr-status",
			Handler:       forwardShowRRStatus,
			PluginCommand: cmdShowRRStatus,
		},
		pluginserver.RPCRegistration{
			WireMethod:    "ze-bgp:show-rr-peers",
			Handler:       forwardShowRRPeers,
			PluginCommand: cmdShowRRPeers,
		},
	)
}

func forwardShowRRStatus(ctx *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	args := validated.Tokens()
	return ctx.Dispatcher().ForwardToPlugin(ctx, cmdShowRRStatus, args, ctx.PeerSelector())
}

func forwardShowRRPeers(ctx *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	args := validated.Tokens()
	return ctx.Dispatcher().ForwardToPlugin(ctx, cmdShowRRPeers, args, ctx.PeerSelector())
}
