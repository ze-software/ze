// Design: docs/architecture/firewall/firewall-domain-group.md -- server-side YANG command forwarding for
// the firewall domain-group plugin. The ze:command nodes in
// yang/ze-firewall-domain-group-cmd.yang need a registered RPC handler each; these
// forwarders hop the command straight to the plugin process via ForwardToPlugin,
// where command.go's handleCommand serves it. Owned by firewall-domain so removing
// the plugin removes the command nodes, these handlers, and the config schema
// together. See ai/rules/plugins.md.

package domain

import (
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// Plugin command names -- shared with the plugin-side CommandDecl and
// handleCommand (domain.go, command.go) so the server forwarders and the plugin
// handlers cannot diverge.
const (
	cmdShowDomainGroup   = "show firewall domain-group"
	cmdUpdateDomainGroup = "update firewall domain-group"
	cmdClearDomainGroup  = "clear firewall domain-group"
)

// leafName is the YANG leaf carrying the group name on all three commands.
// matchCommandTokens treats a key token that names an ArgDef as a typed
// selector keyword, so it binds the value into ctx.Selectors and leaves args
// empty (internal/component/plugin/server/command.go). The leaf here is `name`,
// which the command path does not spell, so the value stays in args; the
// selector is read anyway, because a dispatcher that started binding it would
// otherwise leave every invocation answering with its usage line, which is what
// it did to `update firewall irr asn` until argsOrSelector was added there.
const leafName = "name"

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{
			WireMethod:    "ze-show:firewall-domain-group-status",
			Handler:       forwardShowDomainGroup,
			PluginCommand: cmdShowDomainGroup,
		},
		pluginserver.RPCRegistration{
			WireMethod:    "ze-update:firewall-domain-group",
			Handler:       forwardUpdateDomainGroup,
			PluginCommand: cmdUpdateDomainGroup,
		},
		pluginserver.RPCRegistration{
			WireMethod:    "ze-clear:firewall-domain-group",
			Handler:       forwardClearDomainGroup,
			PluginCommand: cmdClearDomainGroup,
		},
	)
}

func forwardShowDomainGroup(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return ctx.Dispatcher().ForwardToPlugin(ctx, cmdShowDomainGroup, argsOrSelector(ctx, args), ctx.PeerSelector())
}

func forwardUpdateDomainGroup(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return ctx.Dispatcher().ForwardToPlugin(ctx, cmdUpdateDomainGroup, argsOrSelector(ctx, args), ctx.PeerSelector())
}

func forwardClearDomainGroup(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return ctx.Dispatcher().ForwardToPlugin(ctx, cmdClearDomainGroup, argsOrSelector(ctx, args), ctx.PeerSelector())
}

// argsOrSelector returns the positional arguments a plugin command was given,
// recovering the value from the bound selector when the dispatcher consumed it.
// Without it the plugin receives no argument at all and answers with its usage
// line.
func argsOrSelector(ctx *pluginserver.CommandContext, args []string) []string {
	if len(args) > 0 {
		return args
	}
	if value := ctx.Selector(leafName); value != "" {
		return []string{value}
	}
	return args
}
