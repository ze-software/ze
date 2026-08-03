// Design: docs/architecture/api/commands.md — show policy routes proxy handler.
// Owned by the policyroute plugin so that removing it removes the
// `show policy routes` command, its schema, and this handler together. See
// ai/rules/plugins.md.

package policyroute

import (
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

const cmdShowPolicyRoutes = "show policy routes"

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
