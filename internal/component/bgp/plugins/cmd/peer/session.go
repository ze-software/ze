// Design: docs/architecture/api/commands.md — BGP peer session handlers
// Overview: peer.go — BGP peer lifecycle and introspection handlers

package peer

import (
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-plugin:session-peer-ready", Handler: handlePeerSessionReady},
	)
}

// handlePeerSessionReady signals that a peer-specific API process has completed initialization.
//
// ctx.Sender is what the peer's End-of-RIB barrier is keyed on, so it is passed
// through rather than read here: the barrier names the route-pushing processes
// this peer waits for, and only the process that sent this command closes its
// own share of the wait (reactor.Peer.SignalAPIReady).
func handlePeerSessionReady(ctx *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	if ctx.Reactor() != nil && ctx.Peer != "" && ctx.Peer != "*" {
		ctx.Reactor().SignalPeerAPIReady(ctx.Peer, ctx.Sender)
	}
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
			"api": "peer ready acknowledged",
		},
	}, nil
}
