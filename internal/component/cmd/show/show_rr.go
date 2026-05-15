// Design: docs/architecture/api/commands.md — show rr proxy handlers

package show

import (
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

const (
	cmdShowRRStatus = "show rr status"
	cmdShowRRPeers  = "show rr peers"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{
			WireMethod:    "ze-show:rr-status",
			Handler:       forwardShowRRStatus,
			PluginCommand: cmdShowRRStatus,
		},
		pluginserver.RPCRegistration{
			WireMethod:    "ze-show:rr-peers",
			Handler:       forwardShowRRPeers,
			PluginCommand: cmdShowRRPeers,
		},
	)
}

func forwardShowRRStatus(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return ctx.Dispatcher().ForwardToPlugin(ctx, cmdShowRRStatus, args, ctx.PeerSelector())
}

func forwardShowRRPeers(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return ctx.Dispatcher().ForwardToPlugin(ctx, cmdShowRRPeers, args, ctx.PeerSelector())
}
