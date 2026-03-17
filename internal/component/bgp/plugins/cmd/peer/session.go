// Design: docs/architecture/api/commands.md — BGP peer session handlers
// Overview: peer.go — BGP peer lifecycle and introspection handlers

package peer

import (
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-plugin:session-peer-ready", CLICommand: "peer plugin session ready", Handler: handlePeerSessionReady, Help: "Signal peer-specific plugin init complete"},
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
