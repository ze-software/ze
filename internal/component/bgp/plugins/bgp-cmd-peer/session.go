// Design: docs/architecture/api/commands.md — BGP peer session handlers
// Overview: register.go — bgp-cmd-peer plugin registration

package bgpcmdpeer

import (
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-plugin:session-peer-ready", CLICommand: "bgp peer plugin session ready", Handler: handlePeerSessionReady, Help: "Signal peer-specific plugin init complete"},
	)
}

// handlePeerSessionReady signals that a peer-specific API process has completed initialization.
func handlePeerSessionReady(ctx *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	if ctx.Reactor() != nil && ctx.Peer != "" && ctx.Peer != "*" {
		ctx.Reactor().SignalPeerAPIReady(ctx.Peer)
	}
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: map[string]any{
			"api": "peer ready acknowledged",
		},
	}, nil
}
