// Design: docs/architecture/api/commands.md — show static proxy handler.
// Owned by the static plugin so that removing it removes the `show static`
// command, its schema, and this handler together. See
// ai/rules/plugins.md.

package static

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

const cmdShowStatic = "show static"

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{
			WireMethod:    "ze-static:show-static",
			Handler:       forwardShowStatic,
			PluginCommand: cmdShowStatic,
		},
	)
}

func forwardShowStatic(ctx *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	args := validated.Tokens()
	return ctx.Dispatcher().ForwardToPlugin(ctx, cmdShowStatic, args, ctx.PeerSelector())
}
