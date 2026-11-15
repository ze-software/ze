// Design: docs/architecture/firewall/firewall-irr.md -- server-side YANG command forwarding for the
// firewall IRR plugin. The ze:command nodes in yang/ze-firewall-irr-cmd.yang need a
// registered RPC handler each; these forwarders hop the command straight to the
// plugin process via ForwardToPlugin, where command.go's handleCommand serves it.
// Owned by firewall-irr so removing the plugin removes the command nodes, these
// handlers, and the config schema together. See ai/rules/plugins.md.
//
// Fetching and removing a stored list are the irr plugin's `update irr` and
// `clear irr` (internal/plugins/irr/register_commands.go); this plugin answers
// only what it enforces.

package irr

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// Plugin command names -- shared with the plugin-side CommandDecl/handleCommand
// (irr.go, command.go) so the server forwarders and plugin handlers cannot diverge.
const (
	cmdShowIRR       = "show firewall irr"
	cmdShowIRRPrefix = "show firewall irr prefix"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{
			WireMethod:    "ze-firewall:show-irr-status",
			Handler:       forwardShowIRR,
			PluginCommand: cmdShowIRR,
		},
		pluginserver.RPCRegistration{
			WireMethod:    "ze-firewall:show-irr-prefix",
			Handler:       forwardShowIRRPrefix,
			PluginCommand: cmdShowIRRPrefix,
		},
	)
}

func forwardShowIRR(ctx *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	args := validated.Tokens()
	return ctx.Dispatcher().ForwardToPlugin(ctx, cmdShowIRR, args, ctx.PeerSelector())
}

// forwardShowIRRPrefix forwards `show firewall irr prefix <name>`; its leaf is
// `name`, which no key token spells, so the value stays in args.
func forwardShowIRRPrefix(ctx *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	args := validated.Tokens()
	return ctx.Dispatcher().ForwardToPlugin(ctx, cmdShowIRRPrefix, args, ctx.PeerSelector())
}
