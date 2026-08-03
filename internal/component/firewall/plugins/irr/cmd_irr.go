// Design: plan/learned/913-firewall-irr.md -- server-side YANG command forwarding for the
// firewall IRR plugin. The ze:command nodes in yang/ze-firewall-irr-cmd.yang need a
// registered RPC handler each; these forwarders hop the command straight to the
// plugin process via ForwardToPlugin, where command.go's handleCommand serves it.
// Owned by firewall-irr so removing the plugin removes the command nodes, these
// handlers, and the config schema together. See ai/rules/plugins.md.

package irr

import (
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// Plugin command names -- shared with the plugin-side CommandDecl/handleCommand
// (irr.go, command.go) so the server forwarders and plugin handlers cannot diverge.
const (
	cmdShowIRR        = "show firewall irr"
	cmdShowIRRPrefix  = "show firewall irr prefix"
	cmdUpdateIRRAll   = "update firewall irr all"
	cmdUpdateIRRAsn   = "update firewall irr asn"
	cmdUpdateIRRAsSet = "update firewall irr as-set"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{
			WireMethod:    "ze-show:firewall-irr-status",
			Handler:       forwardShowIRR,
			PluginCommand: cmdShowIRR,
		},
		pluginserver.RPCRegistration{
			WireMethod:    "ze-show:firewall-irr-prefix",
			Handler:       forwardShowIRRPrefix,
			PluginCommand: cmdShowIRRPrefix,
		},
		pluginserver.RPCRegistration{
			WireMethod:    "ze-update:firewall-irr-all",
			Handler:       forwardUpdateIRRAll,
			PluginCommand: cmdUpdateIRRAll,
		},
		pluginserver.RPCRegistration{
			WireMethod:    "ze-update:firewall-irr-asn",
			Handler:       forwardUpdateIRRAsn,
			PluginCommand: cmdUpdateIRRAsn,
		},
		pluginserver.RPCRegistration{
			WireMethod:    "ze-update:firewall-irr-as-set",
			Handler:       forwardUpdateIRRAsSet,
			PluginCommand: cmdUpdateIRRAsSet,
		},
	)
}

func forwardShowIRR(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return ctx.Dispatcher().ForwardToPlugin(ctx, cmdShowIRR, args, ctx.PeerSelector())
}

func forwardShowIRRPrefix(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return ctx.Dispatcher().ForwardToPlugin(ctx, cmdShowIRRPrefix, args, ctx.PeerSelector())
}

func forwardUpdateIRRAll(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return ctx.Dispatcher().ForwardToPlugin(ctx, cmdUpdateIRRAll, args, ctx.PeerSelector())
}

func forwardUpdateIRRAsn(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return ctx.Dispatcher().ForwardToPlugin(ctx, cmdUpdateIRRAsn, args, ctx.PeerSelector())
}

func forwardUpdateIRRAsSet(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return ctx.Dispatcher().ForwardToPlugin(ctx, cmdUpdateIRRAsSet, args, ctx.PeerSelector())
}
