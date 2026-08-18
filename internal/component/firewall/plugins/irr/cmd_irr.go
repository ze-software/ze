// Design: docs/architecture/firewall/firewall-irr.md -- server-side YANG command forwarding for the
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
	cmdClearIRRAsn    = "clear firewall irr asn"
	cmdClearIRRAsSet  = "clear firewall irr as-set"
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
		pluginserver.RPCRegistration{
			WireMethod:    "ze-clear:firewall-irr-asn",
			Handler:       forwardClearIRRAsn,
			PluginCommand: cmdClearIRRAsn,
		},
		pluginserver.RPCRegistration{
			WireMethod:    "ze-clear:firewall-irr-as-set",
			Handler:       forwardClearIRRAsSet,
			PluginCommand: cmdClearIRRAsSet,
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
	return ctx.Dispatcher().ForwardToPlugin(ctx, cmdUpdateIRRAsn, argsOrSelector(ctx, args, leafASN), ctx.PeerSelector())
}

func forwardUpdateIRRAsSet(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return ctx.Dispatcher().ForwardToPlugin(ctx, cmdUpdateIRRAsSet, argsOrSelector(ctx, args, leafASSet), ctx.PeerSelector())
}

func forwardClearIRRAsn(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return ctx.Dispatcher().ForwardToPlugin(ctx, cmdClearIRRAsn, argsOrSelector(ctx, args, leafASN), ctx.PeerSelector())
}

func forwardClearIRRAsSet(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return ctx.Dispatcher().ForwardToPlugin(ctx, cmdClearIRRAsSet, argsOrSelector(ctx, args, leafASSet), ctx.PeerSelector())
}

// The four commands above end in the same word as the YANG leaf that carries
// their value: `update firewall irr asn <asn>` reads leaf `asn` under container
// `asn`. matchCommandTokens treats a key token that names an ArgDef as a typed
// selector keyword, so it binds the value into ctx.Selectors and leaves args
// empty (internal/component/plugin/server/command.go, matchCommandTokens).
// `show firewall irr prefix <name>` is unaffected: its leaf is `name`, which no
// key token spells, so its value stays in args.
const (
	leafASN   = "asn"
	leafASSet = "as-set"
)

// argsOrSelector returns the positional arguments a plugin command was given,
// recovering the value from the bound selector when the dispatcher consumed it.
// Without it the plugin receives no argument at all and answers with its usage
// line, which is what made every `update firewall irr asn|as-set` invocation
// fail.
func argsOrSelector(ctx *pluginserver.CommandContext, args []string, leaf string) []string {
	if len(args) > 0 {
		return args
	}
	if value := ctx.Selector(leaf); value != "" {
		return []string{value}
	}
	return args
}
