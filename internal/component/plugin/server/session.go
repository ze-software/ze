// Design: docs/architecture/api/process-protocol.md — plugin process management
// Overview: register.go — RPC registration hub

package server

import (
	"errors"
	"os"

	plugin "github.com/ze-software/ze/internal/component/plugin"
)

// ErrSilent is returned when a command should produce no response.
var ErrSilent = errors.New("silent")

func init() {
	RegisterRPCs(
		RPCRegistration{WireMethod: "ze-plugin:session-ready", Handler: handlePluginSessionReady},
		RPCRegistration{WireMethod: "ze-plugin:session-ping", Handler: handlePluginSessionPing},
		RPCRegistration{WireMethod: "ze-plugin:session-bye", Handler: handlePluginSessionBye},
	)
}

// handlePluginSessionPing answers a health check with the daemon process id,
// under the `pid` output leaf ze-plugin-api.yang declares as uint32. The id is
// what a caller reads to tell one daemon from another after a restart; any
// answer at all says the daemon is alive.
func handlePluginSessionPing(_ *CommandContext, _ []string) (*plugin.Response, error) {
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
			"pid": os.Getpid(),
		},
	}, nil
}

// handlePluginSessionBye handles client disconnect cleanup.
// Called when a client is disconnecting from the API.
func handlePluginSessionBye(_ *CommandContext, _ []string) (*plugin.Response, error) {
	// Currently just acknowledges the disconnect.
	// Future: could clean up client-specific state.
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
			"status": "goodbye",
		},
	}, nil
}

// handlePluginSessionReady signals that an API process has completed initialization.
// Unblocks reactor startup. Peer-specific ready is handled in bgp/plugins/cmd/peer/session.go.
func handlePluginSessionReady(ctx *CommandContext, _ []string) (*plugin.Response, error) {
	if ctx.Reactor() != nil {
		ctx.Reactor().SignalAPIReady()
	}
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
			"api": "ready acknowledged",
		},
	}, nil
}
