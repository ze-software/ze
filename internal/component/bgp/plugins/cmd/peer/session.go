// Design: docs/architecture/api/commands.md — BGP peer session handlers
// Overview: peer.go — BGP peer lifecycle and introspection handlers

package peer

import (
	"fmt"
	"strconv"

	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-bgp:plugin-session-peer-ready", Handler: handlePeerSessionReady},
	)
}

// handlePeerSessionReady signals that a peer-specific API process has completed initialization.
//
// ctx.Sender names the reporter and the token identifies the peer-UP replay
// being completed. Only that reporter's share of the current session's live
// forward fence can be released; EOR publication has its own lifetime.
func handlePeerSessionReady(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	var initialReplay uint64
	if len(args) != 0 {
		if len(args) != 2 || args[0] != "session" {
			return nil, fmt.Errorf("plugin session ready expects session <initial-replay token>")
		}
		var err error
		initialReplay, err = strconv.ParseUint(args[1], 10, 64)
		if err != nil || initialReplay == 0 {
			return nil, fmt.Errorf("plugin session ready requires a nonzero decimal session token from the peer UP event")
		}
	}
	if !ctx.Sender.IsOperator() && initialReplay == 0 {
		return nil, fmt.Errorf("plugin session ready requires session <initial-replay token> from the peer UP event")
	}
	if ctx.Reactor() != nil && ctx.Peer != "" && ctx.Peer != "*" {
		if err := ctx.Reactor().SignalPeerAPIReady(ctx.Peer, ctx.Sender, initialReplay); err != nil {
			return nil, fmt.Errorf("peer %s readiness refused: %w", ctx.Peer, err)
		}
	}
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
			"api": "peer ready acknowledged",
		},
	}, nil
}
