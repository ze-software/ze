// Design: docs/architecture/api/commands.md — show static proxy handler

package show

import (
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

const cmdShowStatic = "show static"

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{
			WireMethod:    "ze-show:static",
			Handler:       forwardShowStatic,
			PluginCommand: cmdShowStatic,
		},
	)
}

func forwardShowStatic(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return ctx.Dispatcher().ForwardToPlugin(ctx, cmdShowStatic, args, ctx.PeerSelector())
}
