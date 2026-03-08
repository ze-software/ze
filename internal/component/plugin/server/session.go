// Design: docs/architecture/api/process-protocol.md — plugin process management
// Overview: register.go — RPC registration hub

package server

import (
	"errors"
	"os"

	plugin "codeberg.org/thomas-mangin/ze/internal/component/plugin"
)

// ErrSilent is returned when a command should produce no response.
var ErrSilent = errors.New("silent")

func init() {
	RegisterRPCs(
		RPCRegistration{WireMethod: "ze-plugin:session-ready", CLICommand: "plugin session ready", Handler: handlePluginSessionReady, Help: "Signal plugin init complete"},
		RPCRegistration{WireMethod: "ze-plugin:session-ping", CLICommand: "plugin session ping", Handler: handlePluginSessionPing, Help: "Health check (returns PID)", ReadOnly: true},
		RPCRegistration{WireMethod: "ze-plugin:session-bye", CLICommand: "plugin session bye", Handler: handlePluginSessionBye, Help: "Disconnect"},
	)
}

// handlePluginSessionPing returns a pong response for health checking.
// Returns daemon PID for identification.
func handlePluginSessionPing(_ *CommandContext, _ []string) (*plugin.Response, error) {
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: map[string]any{
			"pong": os.Getpid(),
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
		Data: map[string]any{
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
		Data: map[string]any{
			"api": "ready acknowledged",
		},
	}, nil
}
