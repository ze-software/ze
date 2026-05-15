// Design: docs/architecture/api/commands.md — show policy-routes proxy handler

package show

import (
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

const cmdShowPolicyRoutes = "show policy-routes"

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{
			WireMethod:    "ze-show:policy-routes",
			Handler:       forwardShowPolicyRoutes,
			PluginCommand: cmdShowPolicyRoutes,
		},
	)
}

func forwardShowPolicyRoutes(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return ctx.Dispatcher().ForwardToPlugin(ctx, cmdShowPolicyRoutes, args, ctx.PeerSelector())
}
