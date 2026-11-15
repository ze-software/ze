// Design: docs/architecture/irr-owner.md -- the daemon forwarders of the irr commands
// Owned by the irr plugin, so removing the plugin removes these command nodes,
// their handlers and the config schema together (ai/rules/plugins.md).

package irr

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// The YANG leaves of the commands that take a value (ze-irr-cmd.yang). Each is
// spelled like the command's last keyword, so the dispatcher binds the value as
// a typed selector and hands the forwarder empty args; ArgsOrSelector reads it
// back (internal/component/plugin/server/command.go).
const (
	leafASN   = "asn"
	leafASSet = "as-set"
)

func init() {
	registerIRRShapes()

	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-irr:show-irr", Handler: forwardShowIRR, PluginCommand: cmdShowIRR},
		pluginserver.RPCRegistration{WireMethod: "ze-irr:update-irr-all", Handler: forwardUpdateIRRAll, PluginCommand: cmdUpdateIRRAll},
		pluginserver.RPCRegistration{WireMethod: "ze-irr:update-irr-asn", Handler: forwardUpdateIRRAsn, PluginCommand: cmdUpdateIRRAsn},
		pluginserver.RPCRegistration{WireMethod: "ze-irr:update-irr-as-set", Handler: forwardUpdateIRRAsSet, PluginCommand: cmdUpdateIRRAsSet, Complete: completeASSetArgument},
		pluginserver.RPCRegistration{WireMethod: "ze-irr:clear-irr-asn", Handler: forwardClearIRRAsn, PluginCommand: cmdClearIRRAsn},
		pluginserver.RPCRegistration{WireMethod: "ze-irr:clear-irr-as-set", Handler: forwardClearIRRAsSet, PluginCommand: cmdClearIRRAsSet, Complete: completeASSetArgument},
	)
}

// completeASSetArgument offers, after `update irr as-set` and `clear irr
// as-set`, every AS-SET name the store holds (completion.go). It runs in the
// daemon beside the forwarders and reads the daemon's store, never the
// owner's process, so it answers while the owner runs forked or is stopped.
// The dispatcher matches the typed prefix (builtinArgCompletions), and the
// SSH tree hangs it on the command's node (WireArgCompletion). Safe for
// concurrent use.
func completeASSetArgument(_ *pluginserver.Server) []pluginserver.Completion {
	known := daemonKnownASSets()
	candidates := make([]pluginserver.Completion, 0, len(known))
	for _, set := range known {
		candidates = append(candidates, pluginserver.Completion{Value: set.name, ShortHelp: set.help()})
	}
	return candidates
}

// registerIRRShapes declares what `show irr` answers: one row per stored
// AS-SET, so the row operators apply. No field is an address (server is a
// host name or host:port), so none is declared and `| resolve` and `| origin`
// are not published for it.
func registerIRRShapes() {
	command.RegisterShape([]string{cmdShowIRR}, command.ShapeTab)
	command.RegisterColumns([]string{cmdShowIRR},
		command.ColumnOrder{
			"as-set", "server", "status", "ipv4-count", "ipv6-count",
			"refreshed-at", "checked-at", "stale-since", "next-refresh", "generation", "consumers",
		},
	)
	command.RegisterAddressFields([]string{cmdShowIRR})
}

func forwardShowIRR(ctx *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	return ctx.Dispatcher().ForwardToPlugin(ctx, cmdShowIRR, validated.Tokens(), ctx.PeerSelector())
}

func forwardUpdateIRRAll(ctx *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	return ctx.Dispatcher().ForwardToPlugin(ctx, cmdUpdateIRRAll, validated.Tokens(), ctx.PeerSelector())
}

func forwardUpdateIRRAsn(ctx *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	return ctx.Dispatcher().ForwardToPlugin(ctx, cmdUpdateIRRAsn, ctx.ArgsOrSelector(validated.Tokens(), leafASN), ctx.PeerSelector())
}

func forwardUpdateIRRAsSet(ctx *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	return ctx.Dispatcher().ForwardToPlugin(ctx, cmdUpdateIRRAsSet, ctx.ArgsOrSelector(validated.Tokens(), leafASSet), ctx.PeerSelector())
}

func forwardClearIRRAsn(ctx *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	return ctx.Dispatcher().ForwardToPlugin(ctx, cmdClearIRRAsn, ctx.ArgsOrSelector(validated.Tokens(), leafASN), ctx.PeerSelector())
}

func forwardClearIRRAsSet(ctx *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	return ctx.Dispatcher().ForwardToPlugin(ctx, cmdClearIRRAsSet, ctx.ArgsOrSelector(validated.Tokens(), leafASSet), ctx.PeerSelector())
}
