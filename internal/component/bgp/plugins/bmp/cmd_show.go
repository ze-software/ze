// Design: docs/architecture/api/commands.md — show bmp proxy handlers.
// Owned by the bgp-bmp plugin so that removing the BMP surface removes the
// `show bmp ...` command, its schema, and these handlers together. See
// ai/rules/plugins.md.

package bmp

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

const (
	cmdShowBMPSessions   = "show bmp sessions"
	cmdShowBMPPeers      = "show bmp peers"
	cmdShowBMPCollectors = "show bmp collectors"
	cmdShowBMPRib        = "show bmp rib"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{
			WireMethod:    "ze-bgp:show-bmp-sessions",
			Handler:       forwardShowBMPSessions,
			PluginCommand: cmdShowBMPSessions,
		},
		pluginserver.RPCRegistration{
			WireMethod:    "ze-bgp:show-bmp-peers",
			Handler:       forwardShowBMPPeers,
			PluginCommand: cmdShowBMPPeers,
		},
		pluginserver.RPCRegistration{
			WireMethod:    "ze-bgp:show-bmp-collectors",
			Handler:       forwardShowBMPCollectors,
			PluginCommand: cmdShowBMPCollectors,
		},
		pluginserver.RPCRegistration{
			WireMethod:    "ze-bgp:show-bmp-rib",
			Handler:       forwardShowBMPRib,
			PluginCommand: cmdShowBMPRib,
		},
	)
}

func forwardShowBMPSessions(ctx *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	args := validated.Tokens()
	return ctx.Dispatcher().ForwardToPlugin(ctx, cmdShowBMPSessions, args, ctx.PeerSelector())
}

func forwardShowBMPPeers(ctx *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	args := validated.Tokens()
	return ctx.Dispatcher().ForwardToPlugin(ctx, cmdShowBMPPeers, args, ctx.PeerSelector())
}

func forwardShowBMPCollectors(ctx *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	args := validated.Tokens()
	return ctx.Dispatcher().ForwardToPlugin(ctx, cmdShowBMPCollectors, args, ctx.PeerSelector())
}

func forwardShowBMPRib(ctx *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	args := validated.Tokens()
	return ctx.Dispatcher().ForwardToPlugin(ctx, cmdShowBMPRib, args, ctx.PeerSelector())
}
