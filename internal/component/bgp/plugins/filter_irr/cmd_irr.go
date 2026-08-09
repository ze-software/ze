// Design: docs/architecture/bgp/filter-irr.md -- YANG command forwarding for IRR filter plugin.
// Owned by bgp-filter-irr so removing the plugin removes these command nodes.

package filter_irr

import (
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

const (
	cmdShowIRR        = "show bgp irr"
	cmdShowIRRPrefix  = "show bgp irr prefix"
	cmdShowIRRCheck   = "show bgp irr check"
	cmdUpdateIRRAll   = "update bgp irr all"
	cmdUpdateIRRAsn   = "update bgp irr asn"
	cmdUpdateIRRAsSet = "update bgp irr as-set"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{
			WireMethod:    "ze-show:irr-status",
			Handler:       forwardShowIRR,
			PluginCommand: cmdShowIRR,
		},
		pluginserver.RPCRegistration{
			WireMethod:    "ze-show:irr-prefix",
			Handler:       forwardShowIRRPrefix,
			PluginCommand: cmdShowIRRPrefix,
		},
		pluginserver.RPCRegistration{
			WireMethod:    "ze-show:irr-check",
			Handler:       forwardShowIRRCheck,
			PluginCommand: cmdShowIRRCheck,
		},
		pluginserver.RPCRegistration{
			WireMethod:    "ze-update:irr-all",
			Handler:       forwardUpdateIRRAll,
			PluginCommand: cmdUpdateIRRAll,
		},
		pluginserver.RPCRegistration{
			WireMethod:    "ze-update:irr-asn",
			Handler:       forwardUpdateIRRAsn,
			PluginCommand: cmdUpdateIRRAsn,
		},
		pluginserver.RPCRegistration{
			WireMethod:    "ze-update:irr-as-set",
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

func forwardShowIRRCheck(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return ctx.Dispatcher().ForwardToPlugin(ctx, cmdShowIRRCheck, args, ctx.PeerSelector())
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
