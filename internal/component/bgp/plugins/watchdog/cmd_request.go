// Design: docs/architecture/api/commands.md -- request bgp watchdog proxy handlers
// Overview: server.go -- the plugin side that answers these commands
// Related: yang/ze-watchdog-cmd.yang -- the command tree these handlers serve
// Related: register.go -- registers these handlers under the YANG wire methods
//
// The YANG tree declares the commands, so the catalog, help and completion
// derive their arguments from it. Each handler forwards to the plugin process,
// which holds the watchdog state and may run external; commandDecls still
// names each command so the engine knows which process serves it.

package watchdog

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// commandRPCs binds each YANG wire method to the handler that forwards it.
func commandRPCs() []pluginserver.RPCRegistration {
	return []pluginserver.RPCRegistration{
		{WireMethod: "ze-bgp:watchdog-announce", Handler: forwardWatchdogAnnounce, PluginCommand: commandRequestWatchdogAnnounce},
		{WireMethod: "ze-bgp:watchdog-withdraw", Handler: forwardWatchdogWithdraw, PluginCommand: commandRequestWatchdogWithdraw},
		{WireMethod: "ze-bgp:watchdog-give", Handler: forwardWatchdogGive, PluginCommand: commandRequestWatchdogGive},
		{WireMethod: "ze-bgp:watchdog-forget", Handler: forwardWatchdogForget, PluginCommand: commandRequestWatchdogForget},
	}
}

func forwardWatchdogAnnounce(ctx *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	return ctx.Dispatcher().ForwardToPlugin(ctx, commandRequestWatchdogAnnounce, validated.Tokens(), ctx.PeerSelector())
}

func forwardWatchdogWithdraw(ctx *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	return ctx.Dispatcher().ForwardToPlugin(ctx, commandRequestWatchdogWithdraw, validated.Tokens(), ctx.PeerSelector())
}

func forwardWatchdogGive(ctx *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	return ctx.Dispatcher().ForwardToPlugin(ctx, commandRequestWatchdogGive, validated.Tokens(), ctx.PeerSelector())
}

func forwardWatchdogForget(ctx *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	return ctx.Dispatcher().ForwardToPlugin(ctx, commandRequestWatchdogForget, validated.Tokens(), ctx.PeerSelector())
}
